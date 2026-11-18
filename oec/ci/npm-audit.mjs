// npm-audit.mjs <ignore list>: judges the JSON report of
// `npm audit --json` (stdin) against an ignore list. Every line of the list
// is an advisory id (GHSA-xxxx-xxxx-xxxx) and the reason why it is accepted;
// "#" starts a comment. An advisory that is not listed fails (exit 1); a
// listed one that npm no longer reports is a warning, so the entry is
// removed when a fix is released. Exit 2: unreadable report or list.
// Used by audit.sh (ADR-0023 §2: an accepted advisory needs an id and a
// reason, logged in the cycle log).
import { readFileSync } from "node:fs";

function fail(msg) {
	console.error(`npm-audit: ${msg}`);
	process.exit(2);
}

if (process.argv.length !== 3) fail("usage: npm-audit.mjs <ignore list> < report.json");
const listFile = process.argv[2];

const ignore = new Map();
readFileSync(listFile, "utf8").split("\n").forEach((raw, i) => {
	const line = raw.replace(/#.*/, "").trim();
	if (line === "") return;
	const m = /^(GHSA(?:-[23456789cfghjmpqrvwx]{4}){3})\s+(\S.*)$/.exec(line);
	if (!m) fail(`${listFile}:${i + 1}: a line needs an advisory id and a reason: ${raw}`);
	ignore.set(m[1], m[2]);
});

let report;
try {
	report = JSON.parse(readFileSync(0, "utf8"));
} catch {
	report = null;
}
if (!report || typeof report.vulnerabilities !== "object" || report.vulnerabilities === null) {
	fail(`no audit report: ${JSON.stringify(report?.error ?? report)}`);
}

const advisories = new Map();
for (const v of Object.values(report.vulnerabilities)) {
	for (const via of v.via ?? []) {
		if (typeof via !== "object" || !via.url) continue;
		const id = via.url.split("/").pop();
		advisories.set(id, via);
	}
}

let bad = 0;
for (const [id, a] of [...advisories].sort()) {
	const reason = ignore.get(id);
	const verdict = reason ? `accepted: ${reason}` : "NOT ACCEPTED";
	if (!reason) bad++;
	console.log(`${id} ${a.severity} ${a.name} ${a.range}: ${a.title} – ${verdict}`);
}
for (const id of ignore.keys()) {
	if (!advisories.has(id)) console.log(`warning: ${id} is no longer reported; remove it from ${listFile}`);
}
console.log(`npm-audit: ${advisories.size} advisories, ${bad} not accepted`);
process.exit(bad ? 1 : 0);
