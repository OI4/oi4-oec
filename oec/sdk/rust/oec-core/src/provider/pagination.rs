//! Pagination of a publication (MG 8.3.11; REQ-PAG-002 … 009): the page
//! plan as a pure function of the DataSetMessage sizes, and the series of
//! NetworkMessages built from it.

use core::ops::Range;

use uuid::Uuid;

use crate::il::IdentificationLink;
use crate::message::{
    DataNetworkMessage, DataSet, DataSetEntry, DataSetMessage, MessageId, Pagination,
    PaginationRequest, PublisherId,
};

/// The pages of a publication.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Plan {
    /// PerPage of the Pagination.
    pub per_page: u32,
    /// The pages to publish: the page number (from 1) and the range of the
    /// resource DataSetMessages it carries.
    pub pages: Vec<(u32, Range<usize>)>,
    /// Whether the NetworkMessages carry a Pagination. Without a
    /// PaginationRequest only a publication that does not fit into one
    /// NetworkMessage is paginated (REQ-PAG-007, REQ-PAG-008).
    pub paginated: bool,
}

/// Plans the pages of `sizes.len()` resource DataSetMessages of the given
/// encoded sizes.
///
/// A NetworkMessage of `k` DataSetMessages is at most
/// `overhead + Σ (size + 1)` bytes, where the overhead is the size of the
/// NetworkMessage without resource DataSetMessages: `single_overhead`
/// without, `page_overhead` with the Pagination helper. Both have to be
/// upper bounds; the message must not exceed `limit` (REQ-PAG-005).
///
/// - Without `request`, one NetworkMessage without Pagination if all fit,
///   else all pages with the largest PerPage that fits (REQ-PAG-007).
/// - With `request`, PerPage is the requested one or, if a page would not
///   fit, the largest smaller one that does (REQ-PAG-004); Page 0 gives all
///   pages, Page `p` the page `p`, empty beyond the last (REQ-PAG-002).
///
/// A DataSetMessage that does not fit even alone gets a page of its own
/// (REQ-PAG-009 is the publisher's duty; the session refuses the message).
pub fn plan(
    sizes: &[usize],
    request: Option<&PaginationRequest>,
    limit: usize,
    single_overhead: usize,
    page_overhead: usize,
) -> Plan {
    let n = sizes.len();
    let cost = |r: Range<usize>| sizes[r].iter().map(|s| s + 1).sum::<usize>();
    let request = request.filter(|r| r.per_page > 0);
    let Some(request) = request else {
        if single_overhead + cost(0..n) <= limit {
            return Plan {
                per_page: to_u32(n.max(1)),
                pages: vec![(1, 0..n)],
                paginated: false,
            };
        }
        let k = largest_fit(sizes, n, limit, page_overhead);
        return Plan {
            per_page: to_u32(k),
            pages: all_pages(n, k),
            paginated: true,
        };
    };
    let wanted = request.per_page as usize;
    let per_page = if fits(sizes, wanted.min(n).max(1), limit, page_overhead) {
        wanted
    } else {
        largest_fit(sizes, wanted.min(n), limit, page_overhead)
    };
    let pages = if request.page == 0 {
        all_pages(n, per_page)
    } else {
        let p = request.page as usize;
        let start = (p - 1).saturating_mul(per_page).min(n);
        vec![(request.page, start..p.saturating_mul(per_page).min(n))]
    };
    Plan {
        per_page: to_u32(per_page),
        pages,
        paginated: true,
    }
}

fn to_u32(n: usize) -> u32 {
    u32::try_from(n).unwrap_or(u32::MAX)
}

/// Whether every page of `k` DataSetMessages fits.
fn fits(sizes: &[usize], k: usize, limit: usize, overhead: usize) -> bool {
    sizes
        .chunks(k)
        .all(|c| overhead + c.iter().map(|s| s + 1).sum::<usize>() <= limit)
}

/// The largest PerPage `≤ bound` whose pages all fit, at least 1.
fn largest_fit(sizes: &[usize], bound: usize, limit: usize, overhead: usize) -> usize {
    let smallest = sizes.iter().min().copied().unwrap_or(0) + 1;
    let bound = bound.min(limit.saturating_sub(overhead) / smallest);
    (1..=bound)
        .rev()
        .find(|k| fits(sizes, *k, limit, overhead))
        .unwrap_or(1)
}

fn all_pages(n: usize, k: usize) -> Vec<(u32, Range<usize>)> {
    if n == 0 {
        return vec![(1, 0..0)];
    }
    (0..n.div_ceil(k))
        .map(|i| (to_u32(i + 1), i * k..((i + 1) * k).min(n)))
        .collect()
}

/// What the NetworkMessages of a publication share.
pub(crate) struct Envelope<'a> {
    pub(crate) publisher: &'a PublisherId,
    /// The publishing application, DataSetWriterName of the helper.
    pub(crate) app: &'a IdentificationLink,
    pub(crate) reply_to: Option<&'a MessageId>,
    pub(crate) limit: usize,
    /// The publisher-defined DataSetClassIds of Data DataSetMessages, one
    /// per item; empty for the other resources. A page carries the class
    /// if all its DataSetMessages share it (REQ-MSG-011).
    pub(crate) classes: &'a [Uuid],
}

/// The encoded NetworkMessages of a publication of `items`, paginated as
/// [`plan`] decides; `next_id` gives the MessageIds.
pub(crate) fn series<P: DataSet>(
    items: Vec<DataSetMessage<P>>,
    request: Option<&PaginationRequest>,
    env: &Envelope<'_>,
    next_id: &mut dyn FnMut() -> MessageId,
) -> Vec<Vec<u8>> {
    let sizes: Vec<usize> = items.iter().map(|m| encode(m).len()).collect();
    let probe = MessageId::new(u64::MAX, Some(u32::MAX), env.publisher);
    let probe_class = env.classes.first().copied();
    let empty = |pagination: bool| {
        let msg = network::<P>(
            probe.clone(),
            env,
            probe_class,
            pagination.then(|| max_pagination(&probe, env)),
        );
        encode(&msg).len()
    };
    let plan = plan(&sizes, request, env.limit, empty(false), empty(true));
    let total = to_u32(items.len());
    let mut items = items.into_iter();
    let mut at = 0;
    let mut first: Option<MessageId> = None;
    let mut out = Vec::with_capacity(plan.pages.len());
    for (page, range) in plan.pages {
        let id = next_id();
        let pagination = plan.paginated.then(|| {
            let series = first.get_or_insert_with(|| id.clone()).clone();
            Pagination {
                total_count: total,
                per_page: plan.per_page,
                page,
                has_next: u64::from(page) * u64::from(plan.per_page) < u64::from(total),
                pagination_id: series,
            }
        });
        let class = env.classes.get(range.clone()).and_then(|c| match c {
            [first, rest @ ..] if rest.iter().all(|r| r == first) => Some(*first),
            _ => None,
        });
        let mut msg = network::<P>(
            id,
            env,
            class,
            pagination.map(|p| DataSetMessage::helper(env.app.clone(), p)),
        );
        let skip = range.start.saturating_sub(at);
        for m in items.by_ref().skip(skip).take(range.len()) {
            msg.push(DataSetEntry::Payload(m));
        }
        at = range.end.max(at);
        out.push(encode(&msg));
    }
    out
}

fn network<P: DataSet>(
    id: MessageId,
    env: &Envelope<'_>,
    class: Option<Uuid>,
    pagination: Option<DataSetMessage<Pagination>>,
) -> DataNetworkMessage<P> {
    let mut msg = DataNetworkMessage::new(id, env.publisher.clone());
    if let Some(c) = class {
        msg = msg
            .with_data_set_class_id(c)
            .expect("the provider checked the DataSetClassId (REQ-MSG-014)");
    }
    if let Some(r) = env.reply_to {
        msg = msg.in_reply_to(r.clone());
    }
    if let Some(p) = pagination {
        msg.push(DataSetEntry::Pagination(p));
    }
    msg
}

fn max_pagination(probe: &MessageId, env: &Envelope<'_>) -> DataSetMessage<Pagination> {
    DataSetMessage::helper(
        env.app.clone(),
        Pagination {
            total_count: u32::MAX,
            per_page: u32::MAX,
            page: u32::MAX,
            has_next: false,
            pagination_id: probe.clone(),
        },
    )
}

/// The types of the message model contain no maps with non-string keys,
/// so they always serialize.
fn encode<T: serde::Serialize>(v: &T) -> Vec<u8> {
    serde_json::to_vec(v).expect("the message model always serializes to JSON")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn req(per_page: u32, page: u32) -> PaginationRequest {
        PaginationRequest { per_page, page }
    }

    #[test]
    fn single_message_without_request() {
        let p = plan(&[10, 10, 10], None, 1000, 100, 200);
        assert_eq!((p.paginated, p.pages), (false, vec![(1, 0..3)]));
    }

    #[test]
    fn paginates_what_does_not_fit() {
        // 4 items of 99 bytes, 100 per item, page overhead 50, limit 260:
        // two per page.
        let p = plan(&[99; 5], None, 260, 40, 50);
        assert!(p.paginated);
        assert_eq!(p.per_page, 2);
        assert_eq!(p.pages, vec![(1, 0..2), (2, 2..4), (3, 4..5)]);
    }

    #[test]
    fn honours_and_lowers_the_requested_page_size() {
        let p = plan(&[10; 5], Some(&req(2, 0)), 1000, 40, 50);
        assert_eq!(
            (p.per_page, p.pages),
            (2, vec![(1, 0..2), (2, 2..4), (3, 4..5)])
        );
        let p = plan(&[10; 5], Some(&req(9, 0)), 1000, 40, 50);
        assert_eq!((p.per_page, p.pages), (9, vec![(1, 0..5)]));
        let p = plan(&[99; 5], Some(&req(4, 2)), 260, 40, 50);
        assert_eq!((p.per_page, p.pages), (2, vec![(2, 2..4)]));
    }

    #[test]
    fn a_page_beyond_the_last_is_empty() {
        let p = plan(&[10; 3], Some(&req(2, 5)), 1000, 40, 50);
        assert_eq!(p.pages, vec![(5, 3..3)]);
        let p = plan(&[], Some(&req(2, 0)), 1000, 40, 50);
        assert_eq!(p.pages, vec![(1, 0..0)]);
    }

    #[test]
    fn oversize_items_get_a_page_each() {
        let p = plan(&[500, 10], None, 260, 40, 50);
        assert_eq!((p.per_page, p.pages), (1, vec![(1, 0..1), (2, 1..2)]));
    }
}
