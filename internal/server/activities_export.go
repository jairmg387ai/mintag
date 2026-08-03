package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// exportSheetName is the fixed sheet name written to every generated
// workbook. Sheet/column/file names are a data contract with downstream
// consumers of the exported file — leave as-is (see design.md "Interfaces").
const exportSheetName = "Actividades"

// exportColumnHeaders are the exact, ordered headers the spec mandates.
// idActividadAzure is a resolved LABEL (see buildActivitiesWorkbook), not a
// raw id.
var exportColumnHeaders = []string{"idActividadAzure", "proyecto", "categoria", "descripcion", "fecha", "tiempo"}

// GET /api/activities/export?from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Returns a downloadable .xlsx of every activity dated within [from, to]
// inclusive, regardless of status. Always resolves idActividadAzure against
// the FULL Azure activity catalog (including inactive rows) so a historical
// row pointing at a since-deactivated activity still shows its real label —
// this is a hard requirement (design D10) distinct from every <select> in
// the app, which must only ever offer active options going forward.
func (srv *Server) handleExportActivities(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	ctx := r.Context()
	activities, err := srv.st.ListActivitiesRange(ctx, from, to, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// includeInactive=true: see the D10 doc comment above — never filter by
	// is_active for the export.
	azureActivities, err := srv.st.ListAzureActivities(ctx, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := buildActivitiesWorkbook(activities, azureActivities)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("actividades_%s_%s.xlsx", from, to)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// buildActivitiesWorkbook renders activities into an in-memory .xlsx with a
// single "Actividades" sheet: a header row followed by one row per activity,
// in the exact column order the spec mandates. az is used only to resolve
// each row's idActividadAzure label (D7) — callers control whether inactive
// rows are included by what they pass in (see handleExportActivities' D10
// comment above and azureLabelByID's doc comment in activities.go).
func buildActivitiesWorkbook(a []*store.DailyActivity, az []*store.AzureActivity) ([]byte, error) {
	labels := azureLabelByID(az)
	defaultLabel, hasDefault := defaultAzureActivityLabel(az, labels)

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", exportSheetName); err != nil {
		return nil, err
	}
	for i, h := range exportColumnHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(exportSheetName, cell, h); err != nil {
			return nil, err
		}
	}

	for i, act := range a {
		row := i + 2 // header occupies row 1
		values := []any{
			resolveExportAzureLabel(act.AzureActivityID, labels, defaultLabel, hasDefault),
			act.Project,
			exportCategoryLabel(act.Category),
			act.RegistroDiario,
			act.Date,
			act.Hours,
		}
		for col, v := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(exportSheetName, cell, v); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func exportCategoryLabel(category string) string {
	idx := strings.LastIndex(category, "/")
	if idx >= 0 && idx < len(category)-1 {
		return category[idx+1:]
	}
	return category
}

// resolveExportAzureLabel implements the design's D7 table for the
// idActividadAzure cell — a Go mirror of
// frontend/src/components/activities/azureActivity.ts:resolveAzureActivityLabel,
// except az/labels here MAY include inactive rows (see D10), so "found"
// covers both active and inactive matches; only a truly dangling id (the
// azure_activities row itself was deleted) falls through to "#<id>".
func resolveExportAzureLabel(id *int64, labels map[int64]string, defaultLabel string, hasDefault bool) string {
	if id == nil {
		if hasDefault {
			return defaultLabel
		}
		return "Default"
	}
	if label, ok := labels[*id]; ok {
		return label
	}
	return fmt.Sprintf("#%d", *id)
}

// defaultAzureActivityLabel finds the current default's formatted label
// (via the shared labels map, so it includes the work item id like every
// other cell) within an already-fetched Azure activity slice, without a
// second store query.
func defaultAzureActivityLabel(az []*store.AzureActivity, labels map[int64]string) (label string, ok bool) {
	for _, item := range az {
		if item.IsDefault {
			return labels[item.ID], true
		}
	}
	return "", false
}
