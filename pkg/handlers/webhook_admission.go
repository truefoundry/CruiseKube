package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/truefoundry/cruisekube/pkg/client"
	"github.com/truefoundry/cruisekube/pkg/logging"

	"github.com/gin-gonic/gin"
	admissionv1 "k8s.io/api/admission/v1"
)

func (deps HandlerDependencies) MutateHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	var review admissionv1.AdmissionReview
	if body, err := c.GetRawData(); err != nil {
		logging.Errorf(ctx, "Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	} else if err := json.Unmarshal(body, &review); err != nil {
		logging.Errorf(ctx, "Failed to unmarshal admission review: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to unmarshal admission review",
		})
		return
	}
	if review.Request == nil {
		logging.Warnf(ctx, "Admission review has no request")
		c.JSON(http.StatusOK, admissionv1.AdmissionReview{
			TypeMeta: review.TypeMeta,
			Response: &admissionv1.AdmissionResponse{
				Allowed: true,
			},
		})
		return
	}
	if deps.RecommenderClient == nil {
		logging.Errorf(ctx, "RecommenderClient not configured")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "webhook not properly configured",
		})
		return
	}

	logging.Infof(ctx, "Forwarding manifest to controller for cluster %s", clusterID)
	mutatingPatchReq := client.MutatingPatchRequest{
		Review: review,
	}
	patchBytes, err := deps.RecommenderClient.WebhookMutatingPatch(ctx, clusterID, mutatingPatchReq)
	if err != nil {
		logging.Errorf(ctx, "Controller mutatingPatch not reachable or error: %v; returning empty patches", err)
		patchBytes = []client.JSONPatchOp{}
	} else if len(patchBytes) == 0 {
		patchBytes = []client.JSONPatchOp{}
	}

	review.Response = &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}
	patch, err := json.Marshal(patchBytes)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal patchBytes: %v", err)
		review.Response.Patch = nil
	} else {
		patchType := admissionv1.PatchTypeJSONPatch
		review.Response.PatchType = &patchType
		review.Response.Patch = patch
	}

	// Return only the response; do not echo the original request.
	responseReview := admissionv1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: review.Response,
	}
	logging.Infof(ctx, "Review response: %s", string(review.Response.Patch))
	c.JSON(http.StatusOK, responseReview)
}
