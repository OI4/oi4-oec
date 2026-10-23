package guideline

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
)

// Item kinds.
const (
	KindSentence = "sentence"
	KindListItem = "list-item"
	KindTableRow = "table-row"
	KindField    = "field"
)

// Bases explain why a text is an inventory candidate.
const (
	BasisKeyword    = "keyword"    // MG 1.2 keyword in the text
	BasisField      = "field"      // field definition with Requirement attribute (MG 1.2)
	BasisAdmonition = "admonition" // tip/warning/info box without keyword in the text (MG 1.2)
	BasisImplicit   = "implicit"   // protocol behavior without keyword (SF-035 interim rule, G-1.6)
	BasisTable      = "table"      // row without keyword in a table that has keyword rows (e.g. "Where applicable")
)

// Item is one candidate of a normative statement.
type Item struct {
	Key        string   `json:"key" yaml:"key"`
	Section    string   `json:"section" yaml:"section"`
	File       string   `json:"file" yaml:"file"`
	Line       int      `json:"line" yaml:"line"`
	Kind       string   `json:"kind" yaml:"kind"`
	Heading    string   `json:"heading,omitempty" yaml:"heading,omitempty"`
	Admonition string   `json:"admonition,omitempty" yaml:"admonition,omitempty"`
	Basis      string   `json:"basis" yaml:"basis"`
	Keywords   []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Terms      []string `json:"terms,omitempty" yaml:"terms,omitempty"`
	Text       string   `json:"text" yaml:"text"`
}

// keywordPattern lists the MG 1.2 keywords; longer alternatives first because
// Go regexp alternation is leftmost-first.
var keywordPattern = regexp.MustCompile(`(?i)\b(shall not|must not|should not|not recommended|highly recommended|best practice|not required|not mandatory|shall|must|required|mandatory|should|recommended|may|optional|can)\b`)

var keywordMap = map[string]string{
	"shall not": "MUST NOT", "must not": "MUST NOT",
	"should not": "SHOULD NOT", "not recommended": "SHOULD NOT",
	"shall": "MUST", "must": "MUST", "required": "MUST", "mandatory": "MUST",
	"should": "SHOULD", "recommended": "SHOULD", "highly recommended": "SHOULD", "best practice": "SHOULD",
	"may": "MAY", "optional": "MAY",
	"can":          "MAY?", // MG 1.2: only as permission – needs a human decision
	"not required": "none", "not mandatory": "none",
}

// implicitPattern finds protocol behavior stated without keyword (SF-035).
var implicitPattern = regexp.MustCompile(`(?i)\b(publish(es|ed)?|answer(s|ed)?|respond(s|ed)?|repl(y|ies|ied)|sends?|sent|reject(s|ed)?|ignor(e|es|ed)|has to|have to|needs? to|is to be|are to be|not allowed|is allowed|are allowed|forbidden|prohibited|cannot|never|always|requires?|expected to)\b`)

// admonitionKeyword maps the box types of MG 1.2 to their obligation.
var admonitionKeyword = map[string]string{"tip": "SHOULD", "warning": "SHOULD", "info": "MUST"}

var (
	fieldPattern    = regexp.MustCompile(`^(Value Range|Type|Access|Requirement|Default Value|Unit)\s*:\s*(.*)$`)
	listPattern     = regexp.MustCompile(`^(\s*)([-*+]|[0-9]+\.)\s+(.*)$`)
	headingPattern  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	captionPattern  = regexp.MustCompile(`^\*?(Figure|Table) [0-9A-Z][0-9.]*:`)
	separatorCell   = regexp.MustCompile(`^:?-+:?$`)
	requirementWord = regexp.MustCompile(`(?i)^(mandatory|optional|not used)`)
)

type tableRow struct {
	line  int
	text  string
	cells string
	cols  []string // cleaned cell per column, "" for empty cells
}

type extractor struct {
	f          File
	items      []Item
	headings   [6]string
	admonition string
	para       []string
	paraLine   int
	list       []string
	listLine   int
	field      []string
	fieldLine  int
	table      []string
	rows       []tableRow
	seen       map[string]int
	heading    string // overrides the heading path of the next emitted item (field tables)
}

// Extract returns all inventory candidates of a file.
func Extract(f File) []Item {
	e := &extractor{f: f, seen: map[string]int{}}
	inFront, inCode, inQuote := false, false, false
	for i, raw := range f.Lines {
		ln := i + 1
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inQuote = false
		}
		// Blockquote content is read like normal text. Only blockquotes that
		// start a block are recognized, so a "> " line inside a paragraph
		// keeps its text.
		if !inCode && strings.HasPrefix(trimmed, ">") && (inQuote || !e.pending()) {
			inQuote = true
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			line = trimmed
		}
		if i == 0 && trimmed == "---" {
			inFront = true
			continue
		}
		if inFront {
			if trimmed == "---" {
				inFront = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			e.flushAll()
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		if strings.HasPrefix(trimmed, ":::") {
			e.flushAll()
			rest := strings.Fields(strings.TrimPrefix(trimmed, ":::"))
			if len(rest) == 0 {
				e.admonition = ""
			} else {
				e.admonition = strings.ToLower(rest[0])
			}
			continue
		}
		if m := headingPattern.FindStringSubmatch(trimmed); m != nil {
			e.flushAll()
			level := len(m[1])
			if level >= 2 {
				e.headings[level-2] = CleanInline(m[2])
				for j := level - 1; j < len(e.headings); j++ {
					e.headings[j] = ""
				}
			}
			continue
		}
		if trimmed == "" {
			e.flushAll()
			continue
		}
		if strings.HasPrefix(trimmed, "|") {
			e.flushPara()
			e.flushList()
			e.flushField()
			e.tableRow(trimmed, ln)
			continue
		}
		e.flushTable()
		if captionPattern.MatchString(trimmed) || strings.HasPrefix(trimmed, "![") || strings.HasPrefix(trimmed, "[![") {
			e.flushAll()
			continue
		}
		if m := fieldPattern.FindStringSubmatch(trimmed); m != nil {
			e.flushPara()
			e.flushList()
			if len(e.field) == 0 {
				e.fieldLine = ln
			}
			e.field = append(e.field, m[1]+": "+CleanInline(m[2]))
			continue
		}
		e.flushField()
		if m := listPattern.FindStringSubmatch(line); m != nil {
			e.flushPara()
			e.flushList()
			e.list = []string{m[3]}
			e.listLine = ln
			continue
		}
		if len(e.list) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			e.list = append(e.list, trimmed)
			continue
		}
		e.flushList()
		if len(e.para) == 0 {
			e.paraLine = ln
		}
		e.para = append(e.para, trimmed)
	}
	e.flushAll()
	return e.items
}

func (e *extractor) pending() bool {
	return len(e.para) > 0 || len(e.list) > 0 || len(e.field) > 0 || e.table != nil
}

func (e *extractor) flushAll() {
	e.flushPara()
	e.flushList()
	e.flushField()
	e.flushTable()
}

// flushTable emits the rows of a table. If any row carries a keyword in its
// cells, all rows are emitted, because rows like "Set | – | Where applicable"
// belong to the same normative table.
func (e *extractor) flushTable() {
	rows, header := e.rows, e.table
	e.rows, e.table = nil, nil
	if isFieldTable(header) {
		e.flushFieldTable(header, rows)
		return
	}
	normative := false
	for _, r := range rows {
		if _, kws := Keywords(r.cells); len(kws) > 0 {
			normative = true
			break
		}
	}
	for _, r := range rows {
		e.emitRow(r, normative)
	}
}

// isFieldTable reports whether a table defines fields: its first column is
// "Field" and it has a "Requirement" column (MG 1.2 field attributes).
func isFieldTable(header []string) bool {
	if len(header) < 2 || !strings.EqualFold(CleanInline(header[0]), "Field") {
		return false
	}
	for _, h := range header[1:] {
		if strings.EqualFold(CleanInline(h), "Requirement") {
			return true
		}
	}
	return false
}

// flushFieldTable emits each row of a field table as a field item, like a
// field definition block below a heading named after the field: the text is
// "<column>: <value>; …" without the Field column and the heading path ends
// with the field name.
func (e *extractor) flushFieldTable(header []string, rows []tableRow) {
	for _, r := range rows {
		if len(r.cols) == 0 || r.cols[0] == "" {
			continue
		}
		e.fieldLine = r.line
		for i := 1; i < len(r.cols) && i < len(header); i++ {
			if r.cols[i] != "" {
				e.field = append(e.field, CleanInline(header[i])+": "+r.cols[i])
			}
		}
		if p := e.headingPath(); p != "" {
			e.heading = p + " / " + r.cols[0]
		} else {
			e.heading = r.cols[0]
		}
		e.flushField()
		e.heading = ""
	}
}

func (e *extractor) flushPara() {
	if len(e.para) == 0 {
		return
	}
	text := CleanInline(strings.Join(e.para, " "))
	for _, s := range SplitSentences(text) {
		e.emit(KindSentence, e.paraLine, s, "")
	}
	e.para = nil
}

func (e *extractor) flushList() {
	if len(e.list) == 0 {
		return
	}
	e.emit(KindListItem, e.listLine, CleanInline(strings.Join(e.list, " ")), "")
	e.list = nil
}

func (e *extractor) flushField() {
	if len(e.field) == 0 {
		return
	}
	req := ""
	for _, a := range e.field {
		if strings.HasPrefix(a, "Requirement: ") {
			req = strings.TrimPrefix(a, "Requirement: ")
		}
	}
	text := strings.Join(e.field, "; ")
	kw := ""
	if m := requirementWord.FindString(req); m != "" {
		kw = map[string]string{"mandatory": "MUST", "optional": "MAY", "not used": "MUST NOT"}[strings.ToLower(m)]
	}
	e.emit(KindField, e.fieldLine, text, kw)
	e.field = nil
}

func (e *extractor) tableRow(row string, ln int) {
	cells := splitCells(row)
	if e.table == nil {
		e.table = cells
		return
	}
	allSep := true
	for _, c := range cells {
		if !separatorCell.MatchString(strings.ReplaceAll(c, " ", "")) {
			allSep = false
			break
		}
	}
	if allSep {
		return
	}
	var parts, values []string
	cols := make([]string, len(cells))
	for i, c := range cells {
		c = CleanInline(c)
		cols[i] = c
		if c == "" {
			continue
		}
		values = append(values, c)
		h := ""
		if i < len(e.table) {
			h = CleanInline(e.table[i])
		}
		if h != "" {
			parts = append(parts, h+": "+c)
		} else {
			parts = append(parts, c)
		}
	}
	if len(parts) > 0 {
		e.rows = append(e.rows, tableRow{line: ln, text: strings.Join(parts, " | "), cells: strings.Join(values, " | "), cols: cols})
	}
}

func splitCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(row); i++ {
		if row[i] == '\\' && i+1 < len(row) && row[i+1] == '|' {
			cur.WriteByte('|')
			i++
			continue
		}
		if row[i] == '|' {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(row[i])
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func (e *extractor) headingPath() string {
	var parts []string
	for _, h := range e.headings {
		if h != "" {
			parts = append(parts, h)
		}
	}
	return strings.Join(parts, " / ")
}

func (e *extractor) emitRow(r tableRow, normative bool) {
	e.emitWith(KindTableRow, r.line, r.text, r.cells, "", normative)
}

func (e *extractor) emit(kind string, line int, text, fieldKeyword string) {
	e.emitWith(kind, line, text, text, fieldKeyword, false)
}

// emitWith classifies text; keywords are searched in kwText (for table rows
// the cell values only, not the column headers).
func (e *extractor) emitWith(kind string, line int, text, kwText, fieldKeyword string, normativeTable bool) {
	if text == "" {
		return
	}
	it := Item{
		Section: e.f.Ref(), File: e.f.Path, Line: line, Kind: kind,
		Heading: e.headingPath(), Admonition: e.admonition, Text: text,
	}
	if e.heading != "" {
		it.Heading = e.heading
	}
	terms, kws := Keywords(kwText)
	if kind == KindField {
		terms, kws = nil, nil
		if fieldKeyword != "" {
			kws = []string{fieldKeyword}
		}
	}
	switch {
	case kind == KindField && len(kws) > 0:
		it.Basis = BasisField
	case kind == KindField:
		return
	case len(kws) > 0:
		it.Basis = BasisKeyword
	case admonitionKeyword[e.admonition] != "":
		it.Basis = BasisAdmonition
		kws = []string{admonitionKeyword[e.admonition]}
	default:
		if m := implicitPattern.FindAllString(kwText, -1); len(m) > 0 {
			it.Basis = BasisImplicit
			terms = uniqueLower(m)
		} else if normativeTable {
			it.Basis = BasisTable
		} else {
			return
		}
	}
	it.Terms, it.Keywords = terms, kws
	it.Key = e.key(it)
	e.items = append(e.items, it)
}

// Keywords returns the MG 1.2 keyword terms found in text and their obligations.
func Keywords(text string) (terms, keywords []string) {
	m := keywordPattern.FindAllString(text, -1)
	terms = uniqueLower(m)
	seen := map[string]bool{}
	for _, t := range terms {
		k := keywordMap[t]
		if !seen[k] {
			seen[k] = true
			keywords = append(keywords, k)
		}
	}
	return terms, keywords
}

func uniqueLower(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.ToLower(s)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// key derives a stable identifier from content, not from line numbers, so
// that unrelated edits of the guideline do not invalidate catalog mappings.
func (e *extractor) key(it Item) string {
	base := strings.Join([]string{e.f.Doc, e.f.Section, it.Kind, it.Heading, NormalizeForCompare(it.Text)}, "\x1f")
	e.seen[base]++
	if n := e.seen[base]; n > 1 {
		base += "\x1f" + strconv.Itoa(n)
	}
	sum := sha256.Sum256([]byte(base))
	return e.f.Doc + "-" + e.f.Section + "-" + hex.EncodeToString(sum[:4])
}
