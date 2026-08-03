package server

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
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

// TestBuildActivitiesWorkbook_ColumnOrder verifies the 6 columns appear in
// exactly the order and with the exact headers the spec mandates, and that a
// data row lands in the right cells.
func TestBuildActivitiesWorkbook_ColumnOrder(t *testing.T) {
	azureID := int64(5)
	activities := []*store.DailyActivity{
		{
			ID: 1, Date: "2026-06-15", Hours: 2.5, Project: "RNCEA",
			Category: "Diseño", RegistroDiario: "RNCEA/Diseño/trabajo hecho",
			Status: "pending", AzureActivityID: &azureID,
		},
	}
	azure := []*store.AzureActivity{
		{ID: 5, Label: "QA Activity", WorkItemID: 4321, IsActive: true},
	}

	data, err := buildActivitiesWorkbook(activities, azure)
	mustNoErr(t, err)

	rows := mustRows(t, openWorkbook(t, data))
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d: %#v", len(rows), rows)
	}
	want := []string{"QA Activity (#4321)", "RNCEA", "Diseño", "RNCEA/Diseño/trabajo hecho", "2026-06-15", "2.5"}
	got := rows[1]
	if len(got) != len(want) {
		t.Fatalf("expected %d data columns, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("col[%d] (%s): expected %q, got %q", i, exportColumnHeaders[i], want[i], got[i])
		}
	}
}

func TestBuildActivitiesWorkbook_CategoryUsesLeafForExport(t *testing.T) {
	tests := []struct {
		name     string
		category string
		want     string
	}{
		{name: "plain category is preserved", category: "Daily", want: "Daily"},
		{name: "hierarchical category exports leaf", category: "Blindaje/Reuniones/Daily", want: "Daily"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			activities := []*store.DailyActivity{
				{ID: 1, Date: "2026-06-15", Hours: 1, Project: "P", Category: tc.category, RegistroDiario: "R", Status: "pending"},
			}
			data, err := buildActivitiesWorkbook(activities, nil)
			mustNoErr(t, err)
			rows := mustRows(t, openWorkbook(t, data))
			if len(rows) != 2 {
				t.Fatalf("expected 2 rows, got %d: %#v", len(rows), rows)
			}
			if rows[1][2] != tc.want {
				t.Errorf("expected categoria=%q, got %q", tc.want, rows[1][2])
			}
		})
	}
}

// TestBuildActivitiesWorkbook_D7LabelCases verifies all 4 label-resolution
// cases from the design's D7 table for the idActividadAzure column.
func TestBuildActivitiesWorkbook_D7LabelCases(t *testing.T) {
	activeID := int64(1)
	inactiveID := int64(2)
	danglingID := int64(999)

	azure := []*store.AzureActivity{
		{ID: 1, Label: "Active Activity", WorkItemID: 101, IsActive: true, IsDefault: false},
		{ID: 2, Label: "Inactive Activity", WorkItemID: 102, IsActive: false, IsDefault: false},
		{ID: 3, Label: "The Default", WorkItemID: 103, IsActive: true, IsDefault: true},
	}

	tests := []struct {
		name string
		id   *int64
		want string
	}{
		{"id set, found and active", &activeID, "Active Activity (#101)"},
		{"id set, found but inactive", &inactiveID, "Inactive Activity (#102)"},
		{"id set, dangling (row deleted outright)", &danglingID, "#999"},
		{"id null, default exists", nil, "The Default (#103)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			activities := []*store.DailyActivity{
				{ID: 1, Date: "2026-06-15", Hours: 1, Project: "P", Category: "C", RegistroDiario: "R", Status: "pending", AzureActivityID: tc.id},
			}
			data, err := buildActivitiesWorkbook(activities, azure)
			mustNoErr(t, err)
			rows := mustRows(t, openWorkbook(t, data))
			if len(rows) != 2 {
				t.Fatalf("expected 2 rows, got %d", len(rows))
			}
			if rows[1][0] != tc.want {
				t.Errorf("expected idActividadAzure=%q, got %q", tc.want, rows[1][0])
			}
		})
	}
}

// TestBuildActivitiesWorkbook_NoDefaultFallsBackToLiteralDefault covers the
// 4th D7 case: id null and no default configured at all.
func TestBuildActivitiesWorkbook_NoDefaultFallsBackToLiteralDefault(t *testing.T) {
	activities := []*store.DailyActivity{
		{ID: 1, Date: "2026-06-15", Hours: 1, Project: "P", Category: "C", RegistroDiario: "R", Status: "pending", AzureActivityID: nil},
	}
	data, err := buildActivitiesWorkbook(activities, nil)
	mustNoErr(t, err)
	rows := mustRows(t, openWorkbook(t, data))
	if rows[1][0] != "Default" {
		t.Errorf("expected literal 'Default', got %q", rows[1][0])
	}
}

// TestBuildActivitiesWorkbook_DeactivatedActivityKeepsRealLabel is the D10
// hard-requirement test: a daily activity referencing an Azure activity that
// has since been deactivated MUST still show its real label in the export —
// never omitted, never a bare #<id>. This is the opposite rule from combos
// (D5), which must never offer an inactive option; the export is a
// historical record and must reflect what actually happened.
func TestBuildActivitiesWorkbook_DeactivatedActivityKeepsRealLabel(t *testing.T) {
	deactivatedID := int64(7)
	activities := []*store.DailyActivity{
		{ID: 1, Date: "2026-06-15", Hours: 1, Project: "P", Category: "C", RegistroDiario: "R", Status: "uploaded", AzureActivityID: &deactivatedID},
	}
	// includeInactive=true shape: the deactivated row is still present in
	// the slice passed to buildActivitiesWorkbook, only its IsActive flag
	// flipped — mirroring what ListAzureActivities(ctx, true) returns.
	azure := []*store.AzureActivity{
		{ID: 7, Label: "Now Deactivated Activity", WorkItemID: 707, IsActive: false},
	}

	data, err := buildActivitiesWorkbook(activities, azure)
	mustNoErr(t, err)
	rows := mustRows(t, openWorkbook(t, data))
	if len(rows) != 2 {
		t.Fatalf("expected the row to be present, got %d rows", len(rows))
	}
	if rows[1][0] != "Now Deactivated Activity (#707)" {
		t.Fatalf("expected the real label to survive deactivation, got %q", rows[1][0])
	}
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

// TestExportActivitiesEndpoint_DeactivatedActivityKeepsRealLabel is the D10
// end-to-end companion to the unit test above: deactivate an Azure activity
// referenced by a historical daily activity, export over REST, assert the
// cell equals the real label.
func TestExportActivitiesEndpoint_DeactivatedActivityKeepsRealLabel(t *testing.T) {
	base, st := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog", map[string]any{
		"org": "RUNT2QA", "work_item_id": 555, "label": "Soon Deactivated",
	})
	assertStatus(t, addResp, http.StatusCreated)
	var az struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, addResp, &az)

	a, err := st.CreateActivity(context.Background(), "2026-06-15", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "referencia historica", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetActivityAzureActivity(context.Background(), a.ID, &az.ID); err != nil {
		t.Fatal(err)
	}

	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/"+strconv.FormatInt(az.ID, 10), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	resp := get(t, base+"/api/activities/export?from=2026-06-01&to=2026-06-30")
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
	if len(rows) != 2 {
		t.Fatalf("expected header + 1 data row, got %d: %#v", len(rows), rows)
	}
	if rows[1][0] != "Soon Deactivated (#555)" {
		t.Fatalf("expected the deactivated activity's real label, got %q", rows[1][0])
	}
}
