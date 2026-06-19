package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	resp, err := h.BillingClient.GetPlans(r.Context())
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	configPlans := make([]billing.Plan, 0, len(h.A.Cfg.Billing.Plans))
	for _, p := range h.A.Cfg.Billing.Plans {
		configPlans = append(configPlans, billing.Plan{ID: p.ID, Name: p.Name, ProductType: p.ProductType})
	}

	mergedPlans := h.mergePlansWithFeatures(resp.Data, configPlans)

	_ = render.Render(w, r, util.NewServerResponse("Plans retrieved successfully", mergedPlans, http.StatusOK))
}

func (h *BillingHandler) mergePlansWithFeatures(plans, configPlans []billing.Plan) []interface{} {
	configPlansMap := make(map[string]map[string]interface{})
	for _, plan := range configPlans {
		if plan.Name == "" {
			continue
		}
		planJSON, err := json.Marshal(plan)
		if err != nil {
			continue
		}
		var planMap map[string]interface{}
		if err := json.Unmarshal(planJSON, &planMap); err != nil {
			continue
		}
		configPlansMap[strings.ToLower(plan.Name)] = planMap
	}

	mergedPlans := make([]interface{}, 0, len(plans))
	for _, plan := range plans {
		planJSON, err := json.Marshal(plan)
		if err != nil {
			continue
		}
		var planMap map[string]interface{}
		if err := json.Unmarshal(planJSON, &planMap); err != nil {
			continue
		}

		configPlanMap, found := configPlansMap[strings.ToLower(plan.Name)]
		if found {
			mergedPlan := make(map[string]interface{})
			for k, v := range configPlanMap {
				mergedPlan[k] = v
			}
			for k, v := range planMap {
				mergedPlan[k] = v
			}
			mergedPlans = append(mergedPlans, mergedPlan)
		} else {
			mergedPlans = append(mergedPlans, planMap)
		}
	}

	return mergedPlans
}

func (h *BillingHandler) GetTaxIDTypes(w http.ResponseWriter, r *http.Request) {
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	resp, err := h.BillingClient.GetTaxIDTypes(r.Context())
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID types retrieved successfully", resp.Data, http.StatusOK))
}
