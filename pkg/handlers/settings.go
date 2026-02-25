package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/types"
)

func GetSettingsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	settings, err := storage.Stg.GetSettings(clusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get settings for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if settings == nil {
		settings = &types.ClusterSettings{
			CPUPricePerCorePerHour:  defaultCPUPricePerCorePerHour,
			MemoryPricePerGBPerHour: defaultMemoryPricePerGBPerHour,
		}
	}

	c.JSON(http.StatusOK, settings)
}

func UpdateSettingsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	var settings types.ClusterSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := storage.Stg.UpdateSettings(clusterID, &settings); err != nil {
		logging.Errorf(ctx, "Failed to update settings for cluster %s: %v", clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logging.Infof(ctx, "Settings updated for cluster %s", clusterID)
	c.JSON(http.StatusOK, settings)
}
