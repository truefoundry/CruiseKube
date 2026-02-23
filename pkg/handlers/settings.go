package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/types"
)

var settingsCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func validateSettingsCrons(settings *types.AppSettings) error {
	for _, expr := range []struct {
		field, value string
	}{
		{"disruptionWindowStartCron", settings.DisruptionWindowStartCron},
		{"disruptionWindowEndCron", settings.DisruptionWindowEndCron},
	} {
		if expr.value == "" {
			continue
		}
		if _, err := settingsCronParser.Parse(expr.value); err != nil {
			return fmt.Errorf("invalid %s %q: %w", expr.field, expr.value, err)
		}
	}
	return nil
}

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
		cpuDefault := float64(defaultCPUPricePerCorePerHour)
		memDefault := float64(defaultMemoryPricePerGbPerHour)
		settings = &types.AppSettings{
			CPUPricePerCorePerHour:  &cpuDefault,
			MemoryPricePerGBPerHour: &memDefault,
		}
	}

	c.JSON(http.StatusOK, settings)
}

func UpdateSettingsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")

	var settings types.AppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validateSettingsCrons(&settings); err != nil {
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
