package ruleschemaobjectpropsexist

import (
	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/simplego/encoding/jsonutil"
	"github.com/grokify/spectrum/openapi3"
	"github.com/grokify/spectrum/openapi3lint/lintutil"
)

type RuleSchemaObjectPropsExist struct {
	name string
}

func NewRule() RuleSchemaObjectPropsExist {
	return RuleSchemaObjectPropsExist{
		name: lintutil.RulenameSchemaObjectPropsExist}
}

func (rule RuleSchemaObjectPropsExist) Name() string {
	return rule.name
}

func (rule RuleSchemaObjectPropsExist) Scope() string {
	return lintutil.ScopeSpecification
}

func (rule RuleSchemaObjectPropsExist) ProcessOperation(spec *openapi3.Spec, op *oas3.Operation, opPointer, path, method string) []lintutil.PolicyViolation {
	return nil
}

func (rule RuleSchemaObjectPropsExist) ProcessSpec(spec *openapi3.Spec, pointerBase string) []lintutil.PolicyViolation {
	vios := []lintutil.PolicyViolation{}

	for schName, schRef := range spec.Components.Schemas {
		if schRef == nil || schRef.Value == nil ||
			schRef.Value.Type != openapi3.TypeObject {
			continue
		}
		if len(schRef.Value.Properties) == 0 &&
			schRef.Value.AdditionalProperties == nil {
			vios = append(vios, lintutil.PolicyViolation{
				RuleName: rule.Name(),
				Location: jsonutil.PointerSubEscapeAll(
					"%s#/components/schemas/%s",
					pointerBase, schName)})
		}
	}
	return vios
}
