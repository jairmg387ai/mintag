package server

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// openWorkbook reads xlsx bytes back with excelize so assertions read real
// cell values instead of trusting the writer blindly.
func openWorkbook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	mustNoErr(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func mustRows(t *testing.T, f *excelize.File) [][]string {
	t.Helper()
	rows, err := f.GetRows(exportSheetName)
	mustNoErr(t, err)
	return rows
}

// TestBuildActivitiesWorkbook_HeaderOnlyOnEmpty verifies an empty activity
// slice still produces a valid workbook containing only the header row, per
// the spec's "Empty range still produces a valid file" scenario.
func TestBuildActivitiesWorkbook_HeaderOnlyOnEmpty(t *testing.T) {
	data, err := buildActivitiesWorkbook(nil, nil)
	mustNoErr(t, err)

	rows := mustRows(t, openWorkbook(t, data))
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (header only), got %d: %#v", len(rows), rows)
	}
	if len(rows[0]) != len(exportColumnHeaders) {
		t.Fatalf("expected %d header columns, got %d: %#v", len(exportColumnHeaders), len(rows[0]), rows[0])
	}
	for i, h := range exportColumnHeaders {
		if rows[0][i] != h {
			t.Errorf("header[%d]: expected %q, got %q", i, h, rows[0][i])
		}
	}
}

// TestBuildActivitiesWorkbook_ColumnOrder verifies the exact 10-column
// header layout and row mapping the external monthly report requires:
// EMPLEADO/DOCUMENTO/CARGO always empty, FECHA INICIO/FECHA FIN both equal
// to the activity's single date, ACTIVIDADES holds the category,
// OBSERVACIONES holds registro_diario, and ID AZURE / MANTIS / LUXFLOW
// follows resolveReportReferenceID's precedence (reference_id, else the
// assigned Azure activity's work item ID, else the catalog default's).
func TestBuildActivitiesWorkbook_ColumnOrder(t *testing.T) {
	wantHeaders := []string{
		"EMPLEADO", "DOCUMENTO", "CARGO", "FECHA INICIO", "FECHA FIN",
		"CANTIDAD (HRS)", "PROYECTO", "ACTIVIDADES", "ID AZURE / MANTIS / LUXFLOW", "OBSERVACIONES",
	}
	if len(exportColumnHeaders) != len(wantHeaders) {
		t.Fatalf("expected %d headers, got %d: %#v", len(wantHeaders), len(exportColumnHeaders), exportColumnHeaders)
	}
	for i, h := range wantHeaders {
		if exportColumnHeaders[i] != h {
			t.Errorf("header[%d]: expected %q, got %q", i, h, exportColumnHeaders[i])
		}
	}

	azure := []*store.AzureActivity{
		{ID: 5, WorkItemID: 4321, Label: "QA Activity", IsActive: true},
		{ID: 9, WorkItemID: 9999, Label: "Default Activity", IsActive: true, IsDefault: true},
	}

	ref := "156789"
	assignedAzureID := int64(5)
	activities := []*store.DailyActivity{
		{
			// reference_id set: wins over any Azure assignment, even though
			// this row has none.
			ID: 1, Date: "2026-06-15", Hours: 2.5, Project: "RNCEA",
			Category: "Diseño", RegistroDiario: "RNCEA/Diseño/trabajo hecho",
			Status: "pending", ReferenceID: &ref,
		},
		{
			// reference_id blank, azure_activity_id set: falls back to that
			// activity's work item ID.
			ID: 2, Date: "2026-06-16", Hours: 1, Project: "RNCEA",
			Category: "Diseño", RegistroDiario: "sin referencia manual",
			Status: "pending", ReferenceID: nil, AzureActivityID: &assignedAzureID,
		},
		{
			// reference_id and azure_activity_id both blank: falls back to
			// the catalog's current default work item ID.
			ID: 3, Date: "2026-06-17", Hours: 3, Project: "RNCEA",
			Category: "Desarrollo", RegistroDiario: "usa la actividad predeterminada",
			Status: "pending",
		},
	}

	data, err := buildActivitiesWorkbook(activities, azure)
	mustNoErr(t, err)

	rows := mustRows(t, openWorkbook(t, data))
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows (header + 3 data), got %d: %#v", len(rows), rows)
	}

	want1 := []string{"", "", "", "2026-06-15", "2026-06-15", "2.5", "RNCEA", "Diseño", "156789", "RNCEA/Diseño/trabajo hecho"}
	assertRow(t, rows[1], want1)

	want2 := []string{"", "", "", "2026-06-16", "2026-06-16", "1", "RNCEA", "Diseño", "4321", "sin referencia manual"}
	assertRow(t, rows[2], want2)

	want3 := []string{"", "", "", "2026-06-17", "2026-06-17", "3", "RNCEA", "Desarrollo", "9999", "usa la actividad predeterminada"}
	assertRow(t, rows[3], want3)
}

func assertRow(t *testing.T, row, want []string) {
	t.Helper()
	got := padRow(row, len(want))
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("col[%d] (%s): expected %q, got %q", i, exportColumnHeaders[i], want[i], got[i])
		}
	}
}

// padRow right-pads a row read back via excelize's GetRows with empty
// strings up to n columns. GetRows trims trailing empty cells per row (a
// documented excelize behavior, not a bug in the writer), so a row whose
// last column(s) are legitimately "" — e.g. OBSERVACIONES, always empty —
// comes back shorter than the sheet's declared column count.
func padRow(row []string, n int) []string {
	if len(row) >= n {
		return row
	}
	padded := make([]string, n)
	copy(padded, row)
	return padded
}

// --- GET /api/activities/export ---

// TestExportActivitiesEndpoint_Success verifies the happy path returns the
// correct content type, a Content-Disposition attachment header with the
// expected filename, and a body excelize can open.
func TestExportActivitiesEndpoint_Success(t *testing.T) {
	base, st := newTestServer(t)

	if _, err := st.CreateActivity(context.Background(), "2026-06-15", 2.0, "RNCEA", "Actividades de arquitectura, diseño y código", "trabajo del rango", "manual"); err != nil {
		t.Fatal(err)
	}

	resp := get(t, base+"/api/activities/export?from=2026-06-01&to=2026-06-30")
	defer resp.Body.Close()

	wantContentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if got := resp.Header.Get("Content-Type"); got != wantContentType {
		t.Errorf("expected Content-Type %q, got %q", wantContentType, got)
	}
	wantDisposition := `attachment; filename="actividades_2026-06-01_2026-06-30.xlsx"`
	if got := resp.Header.Get("Content-Disposition"); got != wantDisposition {
		t.Errorf("expected Content-Disposition %q, got %q", wantDisposition, got)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	mustNoErr(t, err)
	defer f.Close()
	rows, err := f.GetRows(exportSheetName)
	mustNoErr(t, err)
	if len(rows) != 2 {
		t.Fatalf("expected header + 1 data row, got %d: %#v", len(rows), rows)
	}
}

// TestExportActivitiesEndpoint_EmptyRangeStillReturnsValidFile verifies the
// spec's "Empty range still produces a valid file" scenario end-to-end.
func TestExportActivitiesEndpoint_EmptyRangeStillReturnsValidFile(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/activities/export?from=2020-01-01&to=2020-01-31")
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	mustNoErr(t, err)
	defer f.Close()
	rows, err := f.GetRows(exportSheetName)
	mustNoErr(t, err)
	if len(rows) != 1 {
		t.Fatalf("expected header-only row, got %d: %#v", len(rows), rows)
	}
}

// TestExportActivitiesEndpoint_FromAfterToReturns400 verifies from > to is
// rejected as a validation error and no file is generated.
func TestExportActivitiesEndpoint_FromAfterToReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, base+"/api/activities/export?from=2026-06-30&to=2026-06-01", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestExportActivitiesEndpoint_BadDateFormatReturns400 verifies malformed
// date query params are rejected.
func TestExportActivitiesEndpoint_BadDateFormatReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, base+"/api/activities/export?from=30-06-2026&to=2026-06-30", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}
