package benchmark

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// NeedleType defines the category of needle evaluation task.
type NeedleType string

const (
	TypeSingleNeedle NeedleType = "single"
	TypeMultiNeedle  NeedleType = "multi"
	TypeMultiHop     NeedleType = "multi-hop"
)

// Needle represents an atomic fact injected into the context.
type Needle struct {
	ID          string   `json:"id"`
	Domain      string   `json:"domain"`
	Depth       float64  `json:"depth"`
	Fact        string   `json:"fact"`
	Query       string   `json:"query"`
	Answer      string   `json:"answer"`
	Distractors []string `json:"distractors,omitempty"`
}

// MultiHopSpec represents a chained multi-hop reasoning task.
type MultiHopSpec struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Hops      []Needle  `json:"hops"`
	FinalQuery string   `json:"final_query"`
	FinalAnswer string  `json:"final_answer"`
}

// NeedleGenerator generates synthetic, domain-specific needles and distractors.
type NeedleGenerator struct{}

// NewNeedleGenerator creates a new generator instance.
func NewNeedleGenerator() *NeedleGenerator {
	return &NeedleGenerator{}
}

// randomHex generates n bytes of random hexadecimal string.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// randomChoice selects one element from a slice.
func randomChoice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(slice))))
	return slice[n.Int64()]
}

var projectNames = []string{"Aegis", "Orion", "Helios", "Vanguard", "Apex", "Nexus", "Titan", "Specter", "Krypton", "Polaris"}
var divisions = []string{"Enterprise Cloud", "Quantum Systems", "Autonomous Robotics", "Deep Inference", "Cyber Intelligence"}
var cities = []string{"Zurich", "Singapore", "Reykjavik", "Tokyo", "Stockholm", "Montreal", "Munich"}

// GenerateSingle creates a single needle at the specified context depth.
func (g *NeedleGenerator) GenerateSingle(depth float64) Needle {
	proj := randomChoice(projectNames)
	code := fmt.Sprintf("SEC-%s-%s", proj, strings.ToUpper(randomHex(3)))

	fact := fmt.Sprintf("The confidential operational access key for Project %s is %s.", proj, code)
	query := fmt.Sprintf("What is the confidential operational access key for Project %s?", proj)

	// Generate 2 adversarial distractors with different codes/projects
	d1 := fmt.Sprintf("The provisional access key for Project %s was decommissioned and replaced by standard protocol.", proj)
	d2 := fmt.Sprintf("The confidential operational access key for Project %s-Backup is SEC-BACKUP-%s.", proj, strings.ToUpper(randomHex(3)))

	return Needle{
		ID:          randomHex(4),
		Domain:      "operations",
		Depth:       depth,
		Fact:        fact,
		Query:       query,
		Answer:      code,
		Distractors: []string{d1, d2},
	}
}

// GenerateMulti creates multiple independent needles across provided depths.
func (g *NeedleGenerator) GenerateMulti(depths []float64) ([]Needle, string, string) {
	needles := make([]Needle, len(depths))
	var answers []string
	var targets []string

	for i, d := range depths {
		div := divisions[i%len(divisions)]
		val := fmt.Sprintf("%d.%dM", 100+(i*37)%800, (i*13)%10)
		targetID := fmt.Sprintf("%s-FY27", strings.ReplaceAll(div, " ", "-"))

		fact := fmt.Sprintf("The verified operating margin projection for %s is exactly $%s.", targetID, val)
		query := fmt.Sprintf("What is the operating margin projection for %s?", targetID)

		needles[i] = Needle{
			ID:     randomHex(4),
			Domain: "finance",
			Depth:  d,
			Fact:   fact,
			Query:  query,
			Answer: val,
		}
		targets = append(targets, targetID)
		answers = append(answers, fmt.Sprintf("%s: $%s", targetID, val))
	}

	combinedQuery := fmt.Sprintf("Report the operating margin projection for the following divisions in order: %s.", strings.Join(targets, ", "))
	combinedAnswer := strings.Join(answers, "; ")

	return needles, combinedQuery, combinedAnswer
}

// GenerateMultiHop creates a chained multi-hop reasoning task across the specified depths.
// Hop 1: Entity A -> Entity B
// Hop 2: Entity B -> Entity C
// Hop 3: Entity C -> Secret Key
// Query asks about Entity A's final secret key.
func (g *NeedleGenerator) GenerateMultiHop(depths []float64) MultiHopSpec {
	if len(depths) < 2 {
		depths = []float64{0.15, 0.50, 0.85}
	}

	agentA := "Director " + randomChoice([]string{"Vance", "Kovacs", "Sterling", "Cross", "Chen"})
	vaultB := "Vault-" + strings.ToUpper(randomHex(2))
	cityC := randomChoice(cities)
	finalKey := fmt.Sprintf("CIPHER-%s-%d", strings.ToUpper(randomHex(2)), 1000+(len(depths)*777)%8999)

	hops := make([]Needle, 3)

	hops[0] = Needle{
		ID:     randomHex(4),
		Domain: "intelligence",
		Depth:  depths[0],
		Fact:   fmt.Sprintf("Primary clearance protocol designates %s as the exclusive custodian of %s.", agentA, vaultB),
		Query:  fmt.Sprintf("Who is the exclusive custodian of %s?", vaultB),
		Answer: agentA,
	}

	d2 := depths[len(depths)/2]
	hops[1] = Needle{
		ID:     randomHex(4),
		Domain: "intelligence",
		Depth:  d2,
		Fact:   fmt.Sprintf("Physical telemetry records confirm %s maintains its secondary hardware enclave in %s.", vaultB, cityC),
		Query:  fmt.Sprintf("In which city does %s maintain its hardware enclave?", vaultB),
		Answer: cityC,
	}

	d3 := depths[len(depths)-1]
	hops[2] = Needle{
		ID:     randomHex(4),
		Domain: "intelligence",
		Depth:  d3,
		Fact:   fmt.Sprintf("The terminal unlock key for the hardware enclave in %s is %s.", cityC, finalKey),
		Query:  fmt.Sprintf("What is the terminal unlock key for %s?", cityC),
		Answer: finalKey,
	}

	finalQuery := fmt.Sprintf("Based on the protocol records, what is the terminal unlock key for the hardware enclave governed by %s's vault?", agentA)

	return MultiHopSpec{
		ID:          randomHex(4),
		Domain:      "multi-hop",
		Hops:        hops,
		FinalQuery:  finalQuery,
		FinalAnswer: finalKey,
	}
}
