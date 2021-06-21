package openapi3lint

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/simplego/encoding/jsonutil"
	"github.com/grokify/simplego/log/severity"
	"github.com/grokify/simplego/text/stringcase"
	"github.com/grokify/simplego/type/stringsutil"
	"github.com/grokify/spectrum/openapi3"
	"github.com/grokify/spectrum/openapi3lint/lintutil"
)

type Policy struct {
	rules       map[string]Rule
	policyRules map[string]PolicyRule
}

func NewPolicy() Policy {
	return Policy{
		rules:       map[string]Rule{},
		policyRules: map[string]PolicyRule{}}
}

func (pol *Policy) AddRule(rule Rule, sev string, errorOnCollision bool) error {
	ruleName := rule.Name()
	if len(strings.TrimSpace(ruleName)) == 0 {
		return errors.New("rule has no name Policy.AddRule()")
	}
	if !stringcase.IsKebabCase(ruleName) {
		return fmt.Errorf("rule to add name must be in in kebab-case format [%s]", ruleName)
	}
	if errorOnCollision {
		if _, ok := pol.policyRules[ruleName]; ok {
			return fmt.Errorf("duplicate rule [%s] Policy.AddRule()", ruleName)
		}
	}
	canonicalSeverity := severity.SeverityError
	if len(strings.TrimSpace(sev)) > 0 {
		canonicalSeverityTry, err := severity.Parse(sev)
		if err != nil {
			return fmt.Errorf("severity not found [%s] Policy.AddRule()", sev)
		}
		canonicalSeverity = canonicalSeverityTry
	}
	pol.policyRules[ruleName] = PolicyRule{
		Rule:     rule,
		Severity: canonicalSeverity}
	return nil
}

/*
func (pol *Policy) addRuleWithPriorError(rule Rule, sev string, err error) error {
	if err != nil {
		return err
	}
	return pol.AddRule(rule, sev, true)
}
*/
/*
func (pol *Policy) AddRule(rule Rule, errorOnCollision bool) error {
	if len(rule.Name()) == 0 {
		return errors.New("rule to add must have non-empty name")
	}
	if !stringcase.IsKebabCase(rule.Name()) {
		return fmt.Errorf("rule to add name must be in in kebab-case format [%s]", rule.Name())
	}
	if _, ok := pol.rules[rule.Name()]; ok {
		if errorOnCollision {
			return fmt.Errorf("add rule collision for [%s]", rule.Name())
		}
	}
	pol.rules[rule.Name()] = rule
	return nil
}
*/

func (pol *Policy) RuleNames() []string {
	ruleNames := []string{}
	for rn := range pol.rules {
		ruleNames = append(ruleNames, rn)
	}
	sort.Strings(ruleNames)
	return ruleNames
}

func (pol *Policy) ValidateSpec(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	vsets := lintutil.NewPolicyViolationsSets()

	unknownScopes := []string{}
	for _, rule := range pol.rules {
		_, err := lintutil.ParseScope(rule.Scope())
		if err != nil {
			unknownScopes = append(unknownScopes, rule.Scope())
		}
	}
	if len(unknownScopes) > 0 {
		return nil, fmt.Errorf("bad policy: rules have unknown scopes [%s]",
			strings.Join(unknownScopes, ","))
	}

	vsetsOps, err := pol.processRulesOperation(spec, pointerBase, filterSeverity)
	if err != nil {
		return vsets, err
	}
	vsets.UpsertSets(vsetsOps)

	vsetsSpec, err := pol.processRulesSpecification(spec, pointerBase, filterSeverity)
	if err != nil {
		return vsets, err
	}
	vsets.UpsertSets(vsetsSpec)

	return vsets, nil
}

func (pol *Policy) processRulesSpecification(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	if spec == nil {
		return nil, errors.New("cannot process nil spec")
	}
	vsets := lintutil.NewPolicyViolationsSets()

	for _, policyRule := range pol.policyRules {
		if !lintutil.ScopeMatch(lintutil.ScopeSpecification, policyRule.Rule.Scope()) {
			continue
		}
		inclRule, err := severity.SeverityInclude(filterSeverity, policyRule.Severity)
		if err != nil {
			return vsets, err
		}
		// fmt.Printf("FILTER_SEV [%v] ITEM_SEV [%v] INCL [%v]\n", filterSeverity, rule.Severity(), inclRule)
		if inclRule {
			//fmt.Printf("PROC RULE name[%s] scope[%s] sev[%s]\n", rule.Name(), rule.Scope(), rule.Severity())
			vsets.AddViolations(policyRule.Rule.ProcessSpec(spec, pointerBase))
		}
	}
	return vsets, nil
}

func (pol *Policy) processRulesOperation(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	vsets := lintutil.NewPolicyViolationsSets()

	severityErrorRules := []string{}
	unknownSeverities := []string{}

	openapi3.VisitOperations(spec,
		func(path, method string, op *oas3.Operation) {
			if op == nil {
				return
			}
			opPointer := jsonutil.PointerSubEscapeAll(
				"%s#/paths/%s/%s", pointerBase, path, strings.ToLower(method))
			for _, policyRule := range pol.policyRules {
				if !lintutil.ScopeMatch(lintutil.ScopeOperation, policyRule.Rule.Scope()) {
					continue
				}
				//fmt.Printf("HERE [%s] RULE [%s] Scope [%s]\n", path, rule.Name(), rule.Scope())
				inclRule, err := severity.SeverityInclude(filterSeverity, policyRule.Severity)
				//fmt.Printf("INCL_RULE? [%v] RULE [%s]\n", inclRule, rule.Name())
				if err != nil {
					severityErrorRules = append(severityErrorRules, policyRule.Rule.Name())
					unknownSeverities = append(unknownSeverities, policyRule.Severity)
				} else if inclRule {
					vsets.AddViolations(policyRule.Rule.ProcessOperation(spec, op, opPointer, path, method))
				}
			}
		},
	)

	if len(severityErrorRules) > 0 || len(unknownSeverities) > 0 {
		severityErrorRules = stringsutil.Dedupe(severityErrorRules)
		sort.Strings(severityErrorRules)
		return vsets, fmt.Errorf(
			"rules with unknown severities rules[%s] severities[%s] valid[%s]",
			strings.Join(unknownSeverities, ","),
			strings.Join(severityErrorRules, ","),
			strings.Join(severity.Severities(), ","))
	}

	return vsets, nil
}
