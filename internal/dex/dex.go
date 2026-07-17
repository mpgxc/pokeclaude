package dex

import "hash/fnv"

// All returns the full list of species, sorted by name.
func All() []Species { return speciesList }

// Count returns how many species are available.
func Count() int { return len(speciesList) }

// SpeciesFor deterministically maps a session id to a species. The same id
// always yields the same Pokémon, so a session keeps its identity for its whole
// lifetime and across restarts.
func SpeciesFor(sessionID string) Species {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sessionID))
	return speciesList[h.Sum32()%uint32(len(speciesList))]
}
