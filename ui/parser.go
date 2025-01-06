package ui

// parser.go

import (
	"strings"

	"govdupes/internal/models"
)

// parseSearchQuery parses the user’s raw filter string into OR-groups of AND-terms.
// Each AND-term can be positive or negative, and can be a multi-word quoted phrase.
type searchQuery struct {
	// disjunction of conjunctions
	// each entry in orGroups is a set of AND terms
	// e.g. ( [Term("foo"), Term("bar") ] ) OR ( [Term("something")] )
	orGroups []andGroup
}

type andGroup struct {
	terms []term
}

type term struct {
	phrase   string // the actual text to match
	excluded bool   // is it a NOT term?
}

// parseSearchQuery implements a simple parser for the rules:
// - Split on " or " or "|" to get orGroups
// - Within each orGroup, split into terms
// - Terms that start with '-' are excluded
// - Quoted phrases are kept intact
func parseSearchQuery(input string) searchQuery {
	input = strings.TrimSpace(input)
	if input == "" {
		return searchQuery{} // empty query
	}

	// Handle the “OR” logic by splitting on `\s+or\s+` or `|`.
	// Do a quick pass to unify all “|” into the word “ or ” for simplicity.
	// Then split on " or " (case-insensitive).
	normalized := strings.ReplaceAll(input, "|", " or ")
	// Also handle uppercase OR
	normalized = strings.ReplaceAll(normalized, " OR ", " or ")
	parts := splitCaseInsensitive(normalized, " or ")

	var orGroups []andGroup
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Now parse this part as a series of AND terms.
		// Respect quoted phrases and tokenize
		tokens := tokenize(part)
		if len(tokens) == 0 {
			continue
		}
		var ag andGroup
		for _, tok := range tokens {
			t := term{phrase: tok, excluded: false}
			if strings.HasPrefix(tok, "-") && len(tok) > 1 {
				// If it starts with -, remove the dash and mark as excluded
				t.phrase = tok[1:]
				t.excluded = true
			}
			ag.terms = append(ag.terms, t)
		}
		orGroups = append(orGroups, ag)
	}

	return searchQuery{orGroups: orGroups}
}

// splitCaseInsensitive splits a string on a substring ignoring case.
func splitCaseInsensitive(s, sep string) []string {
	var result []string
	sepLower := strings.ToLower(sep)
	sLower := strings.ToLower(s)

	for {
		idx := strings.Index(sLower, sepLower)
		if idx == -1 {
			// no more occurrences
			result = append(result, strings.TrimSpace(s))
			break
		}
		// slice up to idx
		chunk := s[:idx]
		result = append(result, strings.TrimSpace(chunk))

		// advance past the separator
		nextPos := idx + len(sep)
		if nextPos >= len(s) {
			break
		}
		s = s[nextPos:]
		sLower = sLower[nextPos:]
	}
	return result
}

// tokenize handles quoted phrases vs. unquoted words.
// E.g.  foo "bar baz" -qux  =>  ["foo", "bar baz", "-qux"]
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if ch == '"' {
			// Toggle inQuotes
			if inQuotes {
				// close quote
				inQuotes = false
				// end current token
				tokens = appendToken(tokens, current.String())
				current.Reset()
			} else {
				// open quote
				inQuotes = true
				// if current has something, that’s separate
				if current.Len() > 0 {
					tokens = appendToken(tokens, current.String())
					current.Reset()
				}
			}
			continue
		}

		// If we’re not in quotes, and we see space, that ends a token
		if !inQuotes && (ch == ' ' || ch == '\t') {
			if current.Len() > 0 {
				tokens = appendToken(tokens, current.String())
				current.Reset()
			}
			continue
		}

		// Otherwise accumulate
		current.WriteByte(ch)
	}

	// Flush remainder
	if current.Len() > 0 {
		tokens = appendToken(tokens, current.String())
	}

	return tokens
}

// appendToken is a helper to skip empty tokens
func appendToken(tokens []string, t string) []string {
	t = strings.TrimSpace(t)
	if t != "" {
		tokens = append(tokens, t)
	}
	return tokens
}

// applyFilter iterates over all videos in the 2D slice and keeps only
// those that match the query. If a group loses all videos, that group is removed.
func applyFilter(allData [][]*models.VideoData, query searchQuery) [][]*models.VideoData {
	// If query has no orGroups, that means it was empty => no filtering
	if len(query.orGroups) == 0 {
		return allData
	}

	var filtered [][]*models.VideoData

	for _, group := range allData {
		var kept []*models.VideoData
		for _, vd := range group {
			// We check if this single VideoData matches the query
			if matchesQuery(vd, query) {
				kept = append(kept, vd)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, kept)
		}
	}

	return filtered
}

// matchesQuery returns true if the video’s path or fileName
// satisfies the OR-of-AND-terms logic.
func matchesQuery(vd *models.VideoData, query searchQuery) bool {
	if vd == nil {
		return false
	}

	// Combine path + filename into one string or treat them separately as you prefer
	checkStr := vd.Video.Path + " " + vd.Video.FileName
	checkStr = strings.ToLower(checkStr)

	// If at least one OR-group is satisfied, return true
	for _, ag := range query.orGroups {
		if andGroupSatisfied(checkStr, ag) {
			return true
		}
	}

	return false
}

// andGroupSatisfied returns true if ALL terms in the group pass for this string.
func andGroupSatisfied(checkStr string, ag andGroup) bool {
	for _, t := range ag.terms {
		needle := strings.ToLower(t.phrase)
		found := strings.Contains(checkStr, needle)

		// If it’s excluded, we must NOT have found it
		if t.excluded && found {
			return false
		}
		// If it’s positive, we must have found it
		if !t.excluded && !found {
			return false
		}
	}
	return true
}
