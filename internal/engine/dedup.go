package engine

import "strings"

func dedup(candidates []Candidate) []Candidate {
	seen := make(map[string]int)
	var result []Candidate
	for _, c := range candidates {
		normalizedName := strings.TrimSpace(strings.ToLower(c.ArtistName))
		idx, exists := seen[normalizedName]
		if exists {
			if c.SimilarityScore > result[idx].SimilarityScore {
				result[idx] = c
			}
		} else {
			seen[normalizedName] = len(result)
			result = append(result, c)
		}
	}

	return result
}
