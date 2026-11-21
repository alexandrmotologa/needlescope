package benchmark

import (
	"strings"
)

// CorpusDomain defines the background thematic domain of the haystack.
type CorpusDomain string

const (
	DomainFinance    CorpusDomain = "finance"
	DomainLegal      CorpusDomain = "legal"
	DomainTechnical  CorpusDomain = "technical"
	DomainHealthcare CorpusDomain = "healthcare"
)

// CorpusGenerator generates realistic domain-specific background text.
type CorpusGenerator struct {
	tokenizer Tokenizer
}

// NewCorpusGenerator creates a new generator instance with the given tokenizer.
func NewCorpusGenerator(tok Tokenizer) *CorpusGenerator {
	if tok == nil {
		tok = NewFastTokenizer()
	}
	return &CorpusGenerator{tokenizer: tok}
}

var financeParagraphs = []string{
	"Management's discussion and analysis indicates that consolidated revenues increased by 8.4%% year over year, driven primarily by recurring enterprise subscriptions and improved net retention rates across international markets. Operating income before tax expanded to $412.8 million compared to $381.1 million in the preceding fiscal period. Capital expenditures remained disciplined at 4.2%% of net sales, focused on edge computing facility deployments and hardware accelerated inference infrastructure.",
	"In evaluating credit exposure across counterparty derivatives, the audit committee notes that gross mark-to-market positions were secured under bilateral collateralization agreements. Net unrealized derivative gains decreased by $14.2 million due to interest rate curve shifting and foreign currency revaluations. Cash equivalents and marketable securities totaled $1.82 billion as of the close of Q3.",
	"The risk factors disclosed in Note 14 emphasize regulatory compliance obligations across cross-border data transfer corridors. While current litigation liabilities remain within established contingency reserves, changes in sovereign export control frameworks may introduce friction in supply chain lead times for high-bandwidth memory modules and custom silicon ASICs.",
	"Depreciation and amortization schedules reflect an estimated useful life of four years for server rack topologies and seven years for primary facility leasehold improvements. General and administrative expenses declined by 110 basis points as a proportion of total revenue, attributable to organizational automation and consolidated vendor software contracts.",
}

var legalParagraphs = []string{
	"Each party agrees that during the term of this Master Services Agreement and for a period of three (3) years thereafter, confidential technical disclosures, trade secrets, and proprietary source materials shall be held in strict confidence and protected using at least reasonable care. Neither party shall reverse engineer, decompile, or create derivative works from provided binaries without express prior written consent.",
	"In no event shall either party's aggregate liability arising out of or related to this Agreement exceed the total amounts actually paid by Customer in the twelve (12) month period immediately preceding the incident giving rise to liability. This limitation shall not apply to breaches of confidentiality or gross negligence resulting in third-party indemnity claims.",
	"Provider warrants that the enterprise services shall perform in material conformity with the documentation and applicable service level specifications. In the event of a persistent Severity 1 service degradation lasting longer than four consecutive hours, Customer shall receive a pro-rata service credit calculated against the applicable monthly billing tier.",
	"This Agreement shall be governed by and construed in accordance with the laws of the State of Delaware, without giving effect to conflict of laws principles. Any legal suit, action, or proceeding arising out of or relating to this Agreement shall be instituted exclusively in the federal or state courts located in New Castle County.",
}

var technicalParagraphs = []string{
	"The distributed consensus subsystem utilizes an optimized Raft implementation with batched write ahead logging and pipelined snapshot streaming. Node heartbeats are evaluated every 150 milliseconds, with election timeout randomized between 300ms and 600ms to mitigate split-vote cascades during network partitions across availability zones.",
	"Incoming gRPC requests pass through a zero-allocation reverse proxy that enforces token bucket rate limiting and cryptographically signs tracing spans using OpenTelemetry headers. Epoll based socket multiplexing ensures kernel context switches remain minimal under high concurrency connection bursts.",
	"Vector embedding indices are partitioned across memory mapped segment files using hierarchical navigable small world graphs with cosine distance metrics. Pruning routines run concurrently during off-peak windows to maintain recall stability above 98%% at 10ms p99 query latency.",
	"Garbage collection telemetry indicates GC pause durations consistently below 500 microseconds under steady state 40,000 requests per second workloads, enabled by arena allocation pools and pre-sized ring buffers for transient payload serialization.",
}

var healthcareParagraphs = []string{
	"The clinical investigation protocol evaluates patient cohort hemodynamics following multi-agent adjuvant immunotherapy administration. Baseline metabolic panels, renal clearance markers, and electrocardiogram telemetry were recorded at 24-hour intervals throughout the 14-day observation cycle.",
	"Pharmacokinetic profile modeling demonstrates a steady-state distribution volume of 0.38 liters per kilogram, with terminal elimination half-life estimated at 6.8 hours. No adverse hepatotoxicity markers were observed across the randomized double-blind evaluation group.",
	"Laboratory analysis confirms platelet counts remained within the normal physiological range of 150,000 to 450,000 per microliter. Routine urinalysis and serum electrolyte measurements showed stable sodium, potassium, and chloride concentrations throughout the treatment regimen.",
	"Discharge summaries note that patient vital signs were stable on room air with oxygen saturation levels consistently exceeding 98%%. Outpatient follow-up consultations were scheduled for day 30 post-treatment with mandatory repeat contrast-enhanced computed tomography imaging.",
}

// Generate creates a synthetic haystack context matching the target token count.
func (c *CorpusGenerator) Generate(targetTokens int, domain CorpusDomain) string {
	if targetTokens <= 0 {
		return ""
	}

	var sourcePool []string
	switch domain {
	case DomainLegal:
		sourcePool = legalParagraphs
	case DomainTechnical:
		sourcePool = technicalParagraphs
	case DomainHealthcare:
		sourcePool = healthcareParagraphs
	case DomainFinance:
		fallthrough
	default:
		sourcePool = financeParagraphs
	}

	var sb strings.Builder
	// Rough pre-allocation: 4 chars per token
	sb.Grow(targetTokens * 4)

	currentTokens := 0
	paraIdx := 0

	for currentTokens < targetTokens {
		para := sourcePool[paraIdx%len(sourcePool)]
		pTokens := c.tokenizer.CountTokens(para)

		if currentTokens+pTokens > targetTokens {
			// Add partial paragraph to hit token target closely
			trimmed := c.tokenizer.TruncateToTokens(para, targetTokens-currentTokens)
			sb.WriteString(trimmed)
			break
		}

		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(para)
		currentTokens += pTokens + 2 // include paragraph break token estimate
		paraIdx++
	}

	return sb.String()
}
