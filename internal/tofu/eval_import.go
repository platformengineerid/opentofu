package tofu

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/opentofu/internal/addrs"
	"github.com/opentofu/opentofu/internal/lang/marks"
	"github.com/opentofu/opentofu/internal/tfdiags"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func evaluateImportIdExpression(expr hcl.Expression, ctx EvalContext) (string, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics

	if expr == nil {
		return "", diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid import id argument",
			Detail:   "The import ID cannot be null.",
			Subject:  nil,
		})
	}

	// The import expression is declared within the root module
	// We need to explicitly use that context
	ctx = ctx.WithPath(addrs.RootModuleInstance)

	importIdVal, evalDiags := ctx.EvaluateExpr(expr, cty.String, nil)
	diags = diags.Append(evalDiags)

	if importIdVal.IsNull() {
		return "", diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid import id argument",
			Detail:   "The import ID cannot be null.",
			Subject:  expr.Range().Ptr(),
		})
	}

	if !importIdVal.IsKnown() {
		return "", diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid import id argument",
			Detail:   `The import block "id" argument depends on resource attributes that cannot be determined until apply, so OpenTofu cannot plan to import this resource.`, // FIXME and what should I do about that?
			Subject:  expr.Range().Ptr(),
			//	Expression:
			//	EvalContext:
			Extra: diagnosticCausedByUnknown(true),
		})
	}

	if importIdVal.HasMark(marks.Sensitive) {
		return "", diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid import id argument",
			Detail:   "The import ID cannot be sensitive.",
			Subject:  expr.Range().Ptr(),
		})
	}

	var importId string
	err := gocty.FromCtyValue(importIdVal, &importId)
	if err != nil {
		return "", diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid import id argument",
			Detail:   fmt.Sprintf("The import ID value is unsuitable: %s.", err),
			Subject:  expr.Range().Ptr(),
		})
	}

	return importId, diags
}
