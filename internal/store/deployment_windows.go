package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// --- Models ---

type DeploymentWindow struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	State         string    `json:"state"`
	CreatedBy     string    `json:"created_by"`
	PlannedAt     string    `json:"planned_at"`
	DeployedAt    string    `json:"deployed_at"`
	RejectionNote string    `json:"rejection_note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DWTask is a task attached to a deployment window.
type DWTask struct {
	DWID      int64  `json:"dw_id"`
	TaskID    int64  `json:"task_id"`
	Note      string `json:"note"`
	TaskTitle string `json:"task_title,omitempty"`
	TaskStatus string `json:"task_status,omitempty"`
}

// DWRepo is a repository version reference inside a deployment window.
type DWRepo struct {
	ID           int64  `json:"id"`
	DWID         int64  `json:"dw_id"`
	GraphNodeKey string `json:"graph_node_key"`
	Version      string `json:"version"`
	Notes        string `json:"notes"`
}

// DWArtifact is a named artifact (script, config, blob) attached to a window.
type DWArtifact struct {
	ID        int64     `json:"id"`
	DWID      int64     `json:"dw_id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// DWTestScenario is a QA scenario that must be signed off before handoff.
type DWTestScenario struct {
	ID          int64  `json:"id"`
	DWID        int64  `json:"dw_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Expected    string `json:"expected"`
	Result      string `json:"result"`
	SignedOffBy string `json:"signed_off_by"`
	SortOrder   int    `json:"sort_order"`
}

// DeploymentWindowDetail is the full aggregate returned by GetDeploymentWindow.
type DeploymentWindowDetail struct {
	*DeploymentWindow
	Tasks     []DWTask         `json:"tasks"`
	Repos     []DWRepo         `json:"repos"`
	Artifacts []DWArtifact     `json:"artifacts"`
	Scenarios []DWTestScenario `json:"scenarios"`
}

// validTransitions encodes the allowed state machine edges. Any transition
// not listed here is rejected by UpdateDeploymentWindowState.
var validTransitions = map[string][]string{
	"draft":     {"submitted"},
	"submitted": {"approved", "draft"},
	"approved":  {"deployed"},
}

// --- Migration ---

func (s *Store) migrateDeploymentWindows() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS deployment_windows (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		title          TEXT NOT NULL,
		description    TEXT NOT NULL DEFAULT '',
		state          TEXT NOT NULL DEFAULT 'draft',
		created_by     TEXT NOT NULL DEFAULT '',
		planned_at     TEXT NOT NULL DEFAULT '',
		deployed_at    TEXT NOT NULL DEFAULT '',
		rejection_note TEXT NOT NULL DEFAULT '',
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dw_tasks (
		dw_id   INTEGER NOT NULL REFERENCES deployment_windows(id) ON DELETE CASCADE,
		task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		note    TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (dw_id, task_id)
	);

	CREATE TABLE IF NOT EXISTS dw_repos (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		dw_id         INTEGER NOT NULL REFERENCES deployment_windows(id) ON DELETE CASCADE,
		graph_node_key TEXT NOT NULL,
		version       TEXT NOT NULL DEFAULT '',
		notes         TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS dw_artifacts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		dw_id      INTEGER NOT NULL REFERENCES deployment_windows(id) ON DELETE CASCADE,
		kind       TEXT NOT NULL DEFAULT 'other',
		name       TEXT NOT NULL,
		path       TEXT NOT NULL DEFAULT '',
		content    TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dw_test_scenarios (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		dw_id        INTEGER NOT NULL REFERENCES deployment_windows(id) ON DELETE CASCADE,
		title        TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		expected     TEXT NOT NULL DEFAULT '',
		result       TEXT NOT NULL DEFAULT '',
		signed_off_by TEXT NOT NULL DEFAULT '',
		sort_order   INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_dw_tasks_dw_id     ON dw_tasks(dw_id);
	CREATE INDEX IF NOT EXISTS idx_dw_repos_dw_id     ON dw_repos(dw_id);
	CREATE INDEX IF NOT EXISTS idx_dw_artifacts_dw_id ON dw_artifacts(dw_id);
	CREATE INDEX IF NOT EXISTS idx_dw_scenarios_dw_id ON dw_test_scenarios(dw_id);

	CREATE VIRTUAL TABLE IF NOT EXISTS deployment_windows_fts USING fts5(
		title,
		description,
		content='deployment_windows',
		content_rowid='id',
		tokenize='unicode61'
	);

	CREATE TRIGGER IF NOT EXISTS dw_ai AFTER INSERT ON deployment_windows BEGIN
		INSERT INTO deployment_windows_fts(rowid, title, description)
		VALUES (new.id, new.title, new.description);
	END;

	CREATE TRIGGER IF NOT EXISTS dw_au AFTER UPDATE ON deployment_windows BEGIN
		INSERT INTO deployment_windows_fts(deployment_windows_fts, rowid, title, description)
		VALUES ('delete', old.id, old.title, old.description);
		INSERT INTO deployment_windows_fts(rowid, title, description)
		VALUES (new.id, new.title, new.description);
	END;

	CREATE TRIGGER IF NOT EXISTS dw_ad AFTER DELETE ON deployment_windows BEGIN
		INSERT INTO deployment_windows_fts(deployment_windows_fts, rowid, title, description)
		VALUES ('delete', old.id, old.title, old.description);
	END;
	`)
	if err != nil {
		return err
	}
	// Additive column guard for pre-existing installs that may lack rejection_note.
	return s.addColumnIfMissing("deployment_windows", "rejection_note", "rejection_note TEXT NOT NULL DEFAULT ''")
}

// --- CRUD: Deployment Windows ---

func (s *Store) CreateDeploymentWindow(title, description, createdBy, plannedAt string) (*DeploymentWindow, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	res, err := s.db.Exec(
		`INSERT INTO deployment_windows (title, description, created_by, planned_at)
		 VALUES (?, ?, ?, ?)`,
		title, description, createdBy, plannedAt,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getDeploymentWindowRow(id)
}

func (s *Store) GetDeploymentWindow(id int64) (*DeploymentWindowDetail, error) {
	dw, err := s.getDeploymentWindowRow(id)
	if err != nil {
		return nil, err
	}
	detail := &DeploymentWindowDetail{
		DeploymentWindow: dw,
		Tasks:            []DWTask{},
		Repos:            []DWRepo{},
		Artifacts:        []DWArtifact{},
		Scenarios:        []DWTestScenario{},
	}

	// Load tasks with joined title and status.
	taskRows, err := s.db.Query(`
		SELECT dt.dw_id, dt.task_id, dt.note, COALESCE(t.title,''), COALESCE(t.status,'')
		FROM dw_tasks dt
		LEFT JOIN tasks t ON t.id = dt.task_id
		WHERE dt.dw_id = ?
		ORDER BY dt.task_id`, id)
	if err != nil {
		return nil, err
	}
	defer taskRows.Close()
	for taskRows.Next() {
		var dt DWTask
		if err := taskRows.Scan(&dt.DWID, &dt.TaskID, &dt.Note, &dt.TaskTitle, &dt.TaskStatus); err != nil {
			return nil, err
		}
		detail.Tasks = append(detail.Tasks, dt)
	}
	if err := taskRows.Err(); err != nil {
		return nil, err
	}

	// Load repos.
	repoRows, err := s.db.Query(`
		SELECT id, dw_id, graph_node_key, version, notes
		FROM dw_repos WHERE dw_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer repoRows.Close()
	for repoRows.Next() {
		var r DWRepo
		if err := repoRows.Scan(&r.ID, &r.DWID, &r.GraphNodeKey, &r.Version, &r.Notes); err != nil {
			return nil, err
		}
		detail.Repos = append(detail.Repos, r)
	}
	if err := repoRows.Err(); err != nil {
		return nil, err
	}

	// Load artifacts.
	artifactRows, err := s.db.Query(`
		SELECT id, dw_id, kind, name, path, content, created_at
		FROM dw_artifacts WHERE dw_id = ? ORDER BY kind, id`, id)
	if err != nil {
		return nil, err
	}
	defer artifactRows.Close()
	for artifactRows.Next() {
		var a DWArtifact
		if err := artifactRows.Scan(&a.ID, &a.DWID, &a.Kind, &a.Name, &a.Path, &a.Content, &a.CreatedAt); err != nil {
			return nil, err
		}
		detail.Artifacts = append(detail.Artifacts, a)
	}
	if err := artifactRows.Err(); err != nil {
		return nil, err
	}

	// Load test scenarios ordered by sort_order.
	scenarioRows, err := s.db.Query(`
		SELECT id, dw_id, title, description, expected, result, signed_off_by, sort_order
		FROM dw_test_scenarios WHERE dw_id = ? ORDER BY sort_order, id`, id)
	if err != nil {
		return nil, err
	}
	defer scenarioRows.Close()
	for scenarioRows.Next() {
		var sc DWTestScenario
		if err := scenarioRows.Scan(&sc.ID, &sc.DWID, &sc.Title, &sc.Description, &sc.Expected, &sc.Result, &sc.SignedOffBy, &sc.SortOrder); err != nil {
			return nil, err
		}
		detail.Scenarios = append(detail.Scenarios, sc)
	}
	if err := scenarioRows.Err(); err != nil {
		return nil, err
	}

	return detail, nil
}

// ListDeploymentWindows returns summary rows (no child collections).
// Pass state="" to list all windows.
func (s *Store) ListDeploymentWindows(state string) ([]*DeploymentWindow, error) {
	q := `SELECT id, title, description, state, created_by, planned_at, deployed_at,
	             rejection_note, created_at, updated_at
	      FROM deployment_windows`
	args := []any{}
	if state != "" {
		q += ` WHERE state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*DeploymentWindow, 0)
	for rows.Next() {
		dw := &DeploymentWindow{}
		if err := rows.Scan(&dw.ID, &dw.Title, &dw.Description, &dw.State,
			&dw.CreatedBy, &dw.PlannedAt, &dw.DeployedAt,
			&dw.RejectionNote, &dw.CreatedAt, &dw.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, dw)
	}
	return out, rows.Err()
}

// UpdateDeploymentWindowState transitions a window to a new state, enforcing
// the state machine. On draft→submitted the graph edges are upserted.
// On submitted→deployed, deployed_at is stamped. The namespace param is used
// for graph edge creation (auto-detected if empty).
func (s *Store) UpdateDeploymentWindowState(id int64, newState, rejectionNote, namespace string) (*DeploymentWindow, error) {
	dw, err := s.getDeploymentWindowRow(id)
	if err != nil {
		return nil, fmt.Errorf("deployment window not found: %w", err)
	}

	allowed, ok := validTransitions[dw.State]
	if !ok {
		return nil, fmt.Errorf("state %q is terminal; no transitions allowed", dw.State)
	}
	valid := false
	for _, a := range allowed {
		if a == newState {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid transition %q → %q", dw.State, newState)
	}

	// Rejection guard: submitted→draft requires a note.
	if dw.State == "submitted" && newState == "draft" && strings.TrimSpace(rejectionNote) == "" {
		return nil, fmt.Errorf("rejection_note is required when returning to draft")
	}

	deployedAt := dw.DeployedAt
	if newState == "deployed" {
		deployedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Clear rejection note unless we're recording a new one.
	storedNote := ""
	if dw.State == "submitted" && newState == "draft" {
		storedNote = rejectionNote
	}

	_, err = s.db.Exec(`
		UPDATE deployment_windows
		SET state = ?, rejection_note = ?, deployed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		newState, storedNote, deployedAt, id,
	)
	if err != nil {
		return nil, err
	}

	// On draft→submitted, upsert graph edges for each referenced repo.
	if dw.State == "draft" && newState == "submitted" {
		if err := s.syncDeploymentWindowGraphEdges(id, namespace); err != nil {
			// Non-fatal: log the issue as an attribute on the node but don't roll
			// back the state transition. Graph is a soft dependency.
			_ = err
		}
	}

	return s.getDeploymentWindowRow(id)
}

// syncDeploymentWindowGraphEdges upserts one deployment_window graph node and
// one "deploys" edge per dw_repos row. Missing target nodes are skipped (soft FK).
func (s *Store) syncDeploymentWindowGraphEdges(dwID int64, namespace string) error {
	if namespace == "" {
		// Auto-detect: use the first namespace found in the graph.
		namespaces, err := s.GraphNamespaces()
		if err != nil || len(namespaces) == 0 {
			namespace = "default"
		} else {
			namespace = namespaces[0]
		}
	}

	dw, err := s.getDeploymentWindowRow(dwID)
	if err != nil {
		return err
	}

	dwNodeKey := fmt.Sprintf("dw:%d", dwID)
	_, _, err = s.UpsertGraphNode(namespace, "deployment_window", dwNodeKey, dw.Title, dw.Description, map[string]any{
		"state":      dw.State,
		"planned_at": dw.PlannedAt,
		"created_by": dw.CreatedBy,
	})
	if err != nil {
		return err
	}

	rows, err := s.db.Query(`SELECT graph_node_key FROM dw_repos WHERE dw_id = ?`, dwID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var targetKey string
		if err := rows.Scan(&targetKey); err != nil {
			continue
		}
		// Resolve the target node's kind by searching for it; skip if missing.
		targetNode, err := s.GetGraphNodeByKey(namespace, "repo", targetKey)
		if err != nil {
			// Try without kind constraint — look for any kind with that key.
			targetNode, err = s.GetGraphNodeByKey("", "", targetKey)
			if err != nil {
				continue // soft FK: skip missing nodes
			}
		}
		_, _ = s.UpsertGraphEdge(namespace, "deploys", "deployment_window", dwNodeKey, targetNode.Kind, targetKey, nil)
	}
	return rows.Err()
}

// --- CRUD: DW Tasks ---

func (s *Store) AddDWTask(dwID, taskID int64, note string) error {
	if err := s.requireDraftState(dwID); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO dw_tasks (dw_id, task_id, note) VALUES (?, ?, ?)`,
		dwID, taskID, note,
	)
	return err
}

func (s *Store) RemoveDWTask(dwID, taskID int64) error {
	if err := s.requireDraftState(dwID); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM dw_tasks WHERE dw_id = ? AND task_id = ?`, dwID, taskID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %d not attached to deployment window %d", taskID, dwID)
	}
	return nil
}

// --- CRUD: DW Repos ---

func (s *Store) AddDWRepo(dwID int64, graphNodeKey, version, notes string) (*DWRepo, error) {
	if err := s.requireDraftState(dwID); err != nil {
		return nil, err
	}
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	res, err := s.db.Exec(
		`INSERT INTO dw_repos (dw_id, graph_node_key, version, notes) VALUES (?, ?, ?, ?)`,
		dwID, graphNodeKey, version, notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getDWRepo(id)
}

func (s *Store) UpdateDWRepo(id int64, version, notes string) (*DWRepo, error) {
	r, err := s.getDWRepo(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireDraftState(r.DWID); err != nil {
		return nil, err
	}
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	_, err = s.db.Exec(`UPDATE dw_repos SET version = ?, notes = ? WHERE id = ?`, version, notes, id)
	if err != nil {
		return nil, err
	}
	return s.getDWRepo(id)
}

func (s *Store) RemoveDWRepo(id int64) error {
	r, err := s.getDWRepo(id)
	if err != nil {
		return err
	}
	if err := s.requireDraftState(r.DWID); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM dw_repos WHERE id = ?`, id)
	return err
}

// --- CRUD: DW Artifacts ---

func (s *Store) AddDWArtifact(dwID int64, kind, name, path, content string) (*DWArtifact, error) {
	if err := s.requireDraftState(dwID); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("artifact name is required")
	}
	if kind == "" {
		kind = "other"
	}
	res, err := s.db.Exec(
		`INSERT INTO dw_artifacts (dw_id, kind, name, path, content) VALUES (?, ?, ?, ?, ?)`,
		dwID, kind, name, path, content,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getDWArtifact(id)
}

func (s *Store) UpdateDWArtifact(id int64, kind, name, path, content string) (*DWArtifact, error) {
	a, err := s.getDWArtifact(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireDraftState(a.DWID); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("artifact name is required")
	}
	if kind == "" {
		kind = "other"
	}
	_, err = s.db.Exec(
		`UPDATE dw_artifacts SET kind = ?, name = ?, path = ?, content = ? WHERE id = ?`,
		kind, name, path, content, id,
	)
	if err != nil {
		return nil, err
	}
	return s.getDWArtifact(id)
}

func (s *Store) RemoveDWArtifact(id int64) error {
	a, err := s.getDWArtifact(id)
	if err != nil {
		return err
	}
	if err := s.requireDraftState(a.DWID); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM dw_artifacts WHERE id = ?`, id)
	return err
}

// --- CRUD: DW Test Scenarios ---

func (s *Store) AddDWTestScenario(dwID int64, title, description, expected string, sortOrder int) (*DWTestScenario, error) {
	if err := s.requireNonDeployedState(dwID); err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("scenario title is required")
	}
	res, err := s.db.Exec(
		`INSERT INTO dw_test_scenarios (dw_id, title, description, expected, sort_order)
		 VALUES (?, ?, ?, ?, ?)`,
		dwID, title, description, expected, sortOrder,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getDWTestScenario(id)
}

func (s *Store) UpdateDWTestScenario(id int64, title, description, expected string, sortOrder int) (*DWTestScenario, error) {
	sc, err := s.getDWTestScenario(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireNonDeployedState(sc.DWID); err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("scenario title is required")
	}
	_, err = s.db.Exec(
		`UPDATE dw_test_scenarios SET title = ?, description = ?, expected = ?, sort_order = ? WHERE id = ?`,
		title, description, expected, sortOrder, id,
	)
	if err != nil {
		return nil, err
	}
	return s.getDWTestScenario(id)
}

func (s *Store) RemoveDWTestScenario(id int64) error {
	sc, err := s.getDWTestScenario(id)
	if err != nil {
		return err
	}
	if err := s.requireNonDeployedState(sc.DWID); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM dw_test_scenarios WHERE id = ?`, id)
	return err
}

// SignOffTestScenario records an auditor's sign-off (pass/fail) on a scenario.
func (s *Store) SignOffTestScenario(id int64, result, signedOffBy string) (*DWTestScenario, error) {
	sc, err := s.getDWTestScenario(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireNonDeployedState(sc.DWID); err != nil {
		return nil, err
	}
	if result != "pass" && result != "fail" {
		return nil, fmt.Errorf("result must be 'pass' or 'fail', got %q", result)
	}
	_, err = s.db.Exec(
		`UPDATE dw_test_scenarios SET result = ?, signed_off_by = ? WHERE id = ?`,
		result, signedOffBy, id,
	)
	if err != nil {
		return nil, err
	}
	return s.getDWTestScenario(id)
}

// --- Markdown Export ---

// ExportDeploymentWindowMarkdown assembles the full Spanish-language markdown
// document used for handoff to the oversight body (interventoría).
func (s *Store) ExportDeploymentWindowMarkdown(id int64) (string, error) {
	detail, err := s.GetDeploymentWindow(id)
	if err != nil {
		return "", fmt.Errorf("deployment window not found: %w", err)
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Ventana de Despliegue: %s\n\n", detail.Title))
	b.WriteString(fmt.Sprintf("**Estado:** %s | **Fecha planificada:** %s | **Creado por:** %s | **Componentes:** %d\n\n",
		detail.State, detail.PlannedAt, detail.CreatedBy, len(detail.Repos)))

	if detail.Description != "" {
		b.WriteString(detail.Description + "\n\n")
	}

	// Section: bugs resolved (tasks).
	b.WriteString("## Bugs resueltos\n\n")
	if len(detail.Tasks) == 0 {
		b.WriteString("_Sin tareas adjuntas._\n\n")
	} else {
		// Fetch the most recent task_history note per task for root-cause context.
		for _, dt := range detail.Tasks {
			b.WriteString(fmt.Sprintf("- **[%d] %s** (estado: %s)", dt.TaskID, dt.TaskTitle, dt.TaskStatus))
			if dt.Note != "" {
				b.WriteString(fmt.Sprintf(" — %s", dt.Note))
			}
			b.WriteString("\n")

			// Fetch most recent history note.
			var latestNote string
			_ = s.db.QueryRow(`
				SELECT note FROM task_history
				WHERE task_id = ? AND note != ''
				ORDER BY created_at DESC LIMIT 1`, dt.TaskID,
			).Scan(&latestNote)
			if latestNote != "" {
				b.WriteString(fmt.Sprintf("  - Causa raíz: %s\n", latestNote))
			}
		}
		b.WriteString("\n")
	}

	// Section: components to deploy (repos).
	b.WriteString("## Componentes a desplegar\n\n")
	if len(detail.Repos) == 0 {
		b.WriteString("_Sin componentes adjuntos._\n\n")
	} else {
		b.WriteString("| Repositorio | Versión | Notas |\n")
		b.WriteString("|---|---|---|\n")
		for _, r := range detail.Repos {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.GraphNodeKey, r.Version, r.Notes))
		}
		b.WriteString("\n")
	}

	// Section: artifacts grouped by kind.
	b.WriteString("## Artefactos\n\n")
	kindOrder := []string{"db_script", "blob", "config", "other"}
	kindLabel := map[string]string{
		"db_script": "Scripts de Base de Datos",
		"blob":      "Blobs",
		"config":    "Configuración",
		"other":     "Otros",
	}
	anyArtifact := false
	for _, kind := range kindOrder {
		var filtered []DWArtifact
		for _, a := range detail.Artifacts {
			if a.Kind == kind {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		anyArtifact = true
		b.WriteString(fmt.Sprintf("### %s\n\n", kindLabel[kind]))
		for _, a := range filtered {
			line := fmt.Sprintf("- [ ] **%s**", a.Name)
			if a.Path != "" {
				line += fmt.Sprintf(" (`%s`)", a.Path)
			}
			if a.Content != "" {
				line += "\n\n  ```\n  " + strings.ReplaceAll(a.Content, "\n", "\n  ") + "\n  ```"
			}
			b.WriteString(line + "\n\n")
		}
	}
	if !anyArtifact {
		b.WriteString("_Sin artefactos adjuntos._\n\n")
	}

	// Section: test scenarios for auditor sign-off.
	b.WriteString("## Escenarios de prueba para interventoría\n\n")
	if len(detail.Scenarios) == 0 {
		b.WriteString("_Sin escenarios definidos._\n\n")
	} else {
		for i, sc := range detail.Scenarios {
			b.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, sc.Title))
			if sc.Description != "" {
				b.WriteString(fmt.Sprintf("   - Descripción: %s\n", sc.Description))
			}
			if sc.Expected != "" {
				b.WriteString(fmt.Sprintf("   - Resultado esperado: %s\n", sc.Expected))
			}
			if sc.Result != "" {
				b.WriteString(fmt.Sprintf("   - Resultado: %s\n", sc.Result))
			}
			if sc.SignedOffBy != "" {
				b.WriteString(fmt.Sprintf("   - Firmado por: %s\n", sc.SignedOffBy))
			}
			b.WriteString("\n")
		}
	}

	if detail.DeployedAt != "" {
		b.WriteString(fmt.Sprintf("---\n\n**Desplegado el:** %s\n", detail.DeployedAt))
	}

	return b.String(), nil
}

// --- Internal helpers ---

func (s *Store) getDeploymentWindowRow(id int64) (*DeploymentWindow, error) {
	dw := &DeploymentWindow{}
	err := s.db.QueryRow(`
		SELECT id, title, description, state, created_by, planned_at, deployed_at,
		       rejection_note, created_at, updated_at
		FROM deployment_windows WHERE id = ?`, id,
	).Scan(&dw.ID, &dw.Title, &dw.Description, &dw.State,
		&dw.CreatedBy, &dw.PlannedAt, &dw.DeployedAt,
		&dw.RejectionNote, &dw.CreatedAt, &dw.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("deployment window %d not found", id)
	}
	return dw, err
}

func (s *Store) getDWRepo(id int64) (*DWRepo, error) {
	r := &DWRepo{}
	err := s.db.QueryRow(
		`SELECT id, dw_id, graph_node_key, version, notes FROM dw_repos WHERE id = ?`, id,
	).Scan(&r.ID, &r.DWID, &r.GraphNodeKey, &r.Version, &r.Notes)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dw_repo %d not found", id)
	}
	return r, err
}

func (s *Store) getDWArtifact(id int64) (*DWArtifact, error) {
	a := &DWArtifact{}
	err := s.db.QueryRow(
		`SELECT id, dw_id, kind, name, path, content, created_at FROM dw_artifacts WHERE id = ?`, id,
	).Scan(&a.ID, &a.DWID, &a.Kind, &a.Name, &a.Path, &a.Content, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dw_artifact %d not found", id)
	}
	return a, err
}

func (s *Store) getDWTestScenario(id int64) (*DWTestScenario, error) {
	sc := &DWTestScenario{}
	err := s.db.QueryRow(
		`SELECT id, dw_id, title, description, expected, result, signed_off_by, sort_order
		 FROM dw_test_scenarios WHERE id = ?`, id,
	).Scan(&sc.ID, &sc.DWID, &sc.Title, &sc.Description, &sc.Expected, &sc.Result, &sc.SignedOffBy, &sc.SortOrder)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dw_test_scenario %d not found", id)
	}
	return sc, err
}

// requireDraftState returns an error unless the deployment window is in draft.
// Used to guard mutations on tasks, repos, and artifacts.
func (s *Store) requireDraftState(dwID int64) error {
	dw, err := s.getDeploymentWindowRow(dwID)
	if err != nil {
		return err
	}
	if dw.State != "draft" {
		return fmt.Errorf("operation only allowed in draft state; current state is %q", dw.State)
	}
	return nil
}

// requireNonDeployedState returns an error if the window is already deployed.
// Used to guard test scenario mutations (allowed in any pre-deployed state).
func (s *Store) requireNonDeployedState(dwID int64) error {
	dw, err := s.getDeploymentWindowRow(dwID)
	if err != nil {
		return err
	}
	if dw.State == "deployed" {
		return fmt.Errorf("operation not allowed; deployment window is already deployed")
	}
	return nil
}
