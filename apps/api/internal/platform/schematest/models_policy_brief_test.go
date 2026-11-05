package schematest

import (
	policybriefdomain "github.com/garda-inflasi/api/internal/policy-brief/domain"
)

func init() {
	registerModels("policy-brief", &policybriefdomain.PolicyBrief{}, &policybriefdomain.BriefJob{}, &policybriefdomain.BriefVersi{})
}
