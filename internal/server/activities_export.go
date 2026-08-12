package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// exportSheetName is the fixed sheet name written to every generated
// workbook. Sheet/column/file names are a data contract with downstream
// consumers of the exported file — leave as-is (see design.md "Interfaces").
const exportSheetName = "Actividades"

// exportColumnHeaders are the exact, ordered headers the monthly external
// report requires. EMPLEADO, DOCUMENTO, and CARGO are not tracked anywhere
// in Mintag's data model and are always emitted empty. FECHA INICIO/FECHA
// FIN both come from the single activity date (each Mintag activity is a
// single-day entry, so start=end). ACTIVIDADES holds the activity's
// category, while OBSERVACIONES holds its registro_diario description. ID
// AZURE / MANTIS / LUXFLOW is primarily the freeform reference_id field;
// when that's blank it falls back to the Azure work item ID behind the
// activity's "Actividad de Azure" assignment (see resolveReportReferenceID).
var exportColumnHeaders = []string{
	"EMPLEADO",
	"DOCUMENTO",
	"CARGO",
	"FECHA INICIO",
	"FECHA FIN",
	"CANTIDAD (HRS)",
	"PROYECTO",
	"ACTIVIDADES",
	"ID AZURE / MANTIS / LUXFLOW",
	"OBSERVACIONES",
}

// GET /api/activities/export?from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Returns a downloadable .xlsx of every activity dated within [from, to]
// inclusive, regardless of status, matching the exact 10-column layout of
// the external monthly report.
func (srv *Server) handleExportActivities(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	ctx := r.Context()
	activities, err := srv.st.ListActivitiesRange(ctx, from, to, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// includeInactive=true: a historical row pointing at a since-deactivated
	// Azure activity must still resolve to its real work item ID as the
	// reference_id fallback (same reasoning as the old D10 requirement).
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
// in the exact 10-column order the external monthly report mandates.
func buildActivitiesWorkbook(a []*store.DailyActivity, az []*store.AzureActivity) ([]byte, error) {
	workItemIDs := workItemIDByAzureActivityID(az)
	defaultWorkItemID, hasDefault := defaultAzureWorkItemID(az, workItemIDs)

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
			"",           // EMPLEADO — not tracked
			"",           // DOCUMENTO — not tracked
			"",           // CARGO — not tracked
			act.Date,     // FECHA INICIO
			act.Date,     // FECHA FIN (single-day entries, so start=end)
			act.Hours,    // CANTIDAD (HRS)
			act.Project,  // PROYECTO
			act.Category, // ACTIVIDADES
			resolveReportReferenceID(act.ReferenceID, act.AzureActivityID, workItemIDs, defaultWorkItemID, hasDefault), // ID AZURE / MANTIS / LUXFLOW
			act.RegistroDiario, // OBSERVACIONES
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

// resolveReportReferenceID implements the ID AZURE / MANTIS / LUXFLOW
// column's precedence: the freeform reference_id wins whenever it's set to
// a non-blank value (it may hold a Mantis ticket, a LuxFlow number, or a
// manually-entered Azure ID); otherwise it falls back to the Azure work
// item ID behind the activity's azure_activity_id assignment — the explicit
// assignment if one exists, or the catalog's current default when the
// activity has none (mirroring how "Actividad de Azure" behaves everywhere
// else: unset means "use the default").
func resolveReportReferenceID(referenceID *string, azureActivityID *int64, workItemIDs map[int64]string, defaultWorkItemID string, hasDefault bool) string {
	if referenceID != nil {
		if trimmed := strings.TrimSpace(*referenceID); trimmed != "" {
			return trimmed
		}
	}
	if azureActivityID != nil {
		return workItemIDs[*azureActivityID]
	}
	if hasDefault {
		return defaultWorkItemID
	}
	return ""
}

// workItemIDByAzureActivityID indexes a list of Azure activities by id,
// resolving each to its raw Azure work item ID (not the "Label (#id)"
// display format — this feeds a plain identifier cell, not a UI label).
func workItemIDByAzureActivityID(az []*store.AzureActivity) map[int64]string {
	out := make(map[int64]string, len(az))
	for _, a := range az {
		out[a.ID] = strconv.Itoa(a.WorkItemID)
	}
	return out
}

// defaultAzureWorkItemID finds the current default Azure activity's work
// item ID within an already-fetched slice, without a second store query.
func defaultAzureWorkItemID(az []*store.AzureActivity, workItemIDs map[int64]string) (id string, ok bool) {
	for _, item := range az {
		if item.IsDefault {
			return workItemIDs[item.ID], true
		}
	}
	return "", false
}
