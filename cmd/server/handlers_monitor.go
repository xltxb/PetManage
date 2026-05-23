package main

import (
	"encoding/json"
	"net/http"

	"github.com/xltxb/PetManage/internal/monitor"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makeMonitorEndpointsHandler(svc *monitor.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "24h"
		}
		keyword := r.URL.Query().Get("keyword")
		sortBy := r.URL.Query().Get("sort_by")
		sortDir := r.URL.Query().Get("sort_dir")

		metrics, err := svc.GetEndpointMetrics(period, keyword, sortBy, sortDir)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query endpoint metrics", err))
			return
		}
		if metrics == nil {
			metrics = []monitor.EndpointMetric{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}

func makeMonitorDevelopersHandler(svc *monitor.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "24h"
		}

		metrics, err := svc.GetDeveloperMetrics(period)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query developer metrics", err))
			return
		}
		if metrics == nil {
			metrics = []monitor.DeveloperMetric{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}

func makeMonitorAnomaliesHandler(svc *monitor.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "24h"
		}

		metrics, err := svc.GetAnomalies(period)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query anomalies", err))
			return
		}
		if metrics == nil {
			metrics = []monitor.EndpointMetric{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}
