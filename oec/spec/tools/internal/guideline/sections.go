package guideline

import "strings"

// SectionTexts maps each section ref ("MG 7.2.1") to its normalized full text
// (front matter removed), used to verify verbatim quotes.
func SectionTexts(files []File) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		lines := f.Lines
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "---" {
					lines = lines[i+1:]
					break
				}
			}
		}
		var b strings.Builder
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "#") {
				l = strings.TrimLeft(l, "# ")
			}
			l = strings.TrimPrefix(l, ":::")
			b.WriteString(l)
			b.WriteByte(' ')
		}
		ref := f.Ref()
		if prev, ok := out[ref]; ok {
			out[ref] = prev + " " + NormalizeForCompare(b.String())
		} else {
			out[ref] = NormalizeForCompare(b.String())
		}
	}
	return out
}
