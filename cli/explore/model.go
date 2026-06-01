package explore

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

// screen is the active full-screen view. Editor, confirm, name, and
// result are modal overlays that remember the screen to return to.
type screen int

const (
	screenProfiles screen = iota
	screenKinds
	screenList
	screenDetail
	screenEditor
	screenConfirm
	screenName
	screenResult
)

// editAction distinguishes the two ways the JSON editor is used.
type editAction int

const (
	actionEdit editAction = iota
	actionCreate
)

// navFrame is one level of the drill-down stack: the parent kind and
// the list position to restore when the user backs out.
type navFrame struct {
	kind   Kind
	rows   []Row
	cursor int
}

// model is the root bubbletea model for `cinc explore`.
type model struct {
	ctx         context.Context
	opts        Options
	client      *cinc.Client // nil until a profile is chosen
	profileName string

	// server identity shown in the title bar
	serverHost    string // hostname[:port] of the connected server
	serverVersion string // e.g. "API v2"; empty until the probe returns

	screen screen
	width  int
	height int

	// profile picker
	profileNames  []string
	profileCursor int

	// kind menu
	kinds      []Kind
	kindCursor int

	// drill-down stack and current list view
	stack   []navFrame
	cur     Kind
	rows    []Row // unfiltered rows for cur
	cursor  int   // index into the filtered rows
	loading bool
	listErr string

	// list filtering
	filtering bool
	filter    textinput.Model

	// detail view
	detail      viewport.Model
	detailReady bool
	detailName  string
	detailErr   string

	// editor modal
	editor   jsoneditor.Model
	editKind editAction
	editName string

	// returnTo is the screen a modal returns to when cancelled.
	returnTo screen

	// confirm modal
	confirmPrompt string
	pending       func() tea.Cmd

	// name-prompt modal
	namePrompt string
	nameInput  textinput.Model
	nameAction func(string) tea.Cmd

	// result modal (one-time secrets etc.)
	resultTitle string
	resultBody  string

	// reqID stamps every async request; stale responses are dropped.
	// Bubbletea dispatches Update serially so a plain counter is safe.
	reqID uint64

	status string // transient success line shown in the footer
	styles styles
	keys   keyMap
}

// startup carries the resolved entry state from Run into the model.
type startup struct {
	client       *cinc.Client
	profileName  string
	screen       screen
	profileNames []string
}

func newModel(ctx context.Context, opts Options, s startup) model {
	filter := textinput.New()
	filter.Prompt = "/"
	filter.Placeholder = "filter…"

	name := textinput.New()
	name.Prompt = "› "

	return model{
		ctx:          ctx,
		opts:         opts,
		client:       s.client,
		profileName:  s.profileName,
		serverHost:   hostFromProfile(opts, s.profileName),
		screen:       s.screen,
		profileNames: s.profileNames,
		kinds:        registry(),
		filter:       filter,
		nameInput:    name,
		styles:       newStyles(),
		keys:         newKeyMap(),
	}
}

// hostFromProfile pulls the server hostname[:port] out of a profile's
// server URL, for display in the title bar. It returns "" when the
// profile or URL is missing or unparseable.
func hostFromProfile(opts Options, name string) string {
	p, ok := opts.Profiles[name]
	if !ok {
		return ""
	}
	u, err := url.Parse(p.ServerURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

func (m *model) nextReqID() uint64 {
	m.reqID++
	return m.reqID
}

// ----- messages --------------------------------------------------------

type clientReadyMsg struct {
	name   string
	client *cinc.Client
	err    error
}

type listLoadedMsg struct {
	reqID uint64
	rows  []Row
	err   error
}

type detailLoadedMsg struct {
	reqID uint64
	name  string
	body  string
	err   error
}

type editSeedMsg struct {
	reqID uint64
	name  string
	json  string
	err   error
}

type mutationDoneMsg struct {
	verb   string // "Updated", "Created", "Deleted"
	name   string
	secret string
	err    error
}

type downloadDoneMsg struct {
	name string
	path string
	err  error
}

type serverInfoMsg struct {
	version string
}

// ----- commands --------------------------------------------------------

func buildClientCmd(opts Options, name string, profile config.Profile) tea.Cmd {
	return func() tea.Msg {
		c, err := opts.NewClient(profile)
		return clientReadyMsg{name: name, client: c, err: err}
	}
}

func listCmd(ctx context.Context, c *cinc.Client, k Kind, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		rows, err := k.List(ctx, c)
		return listLoadedMsg{reqID: reqID, rows: rows, err: err}
	}
}

func describeCmd(ctx context.Context, c *cinc.Client, v Viewable, name string, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		body, err := v.Describe(ctx, c, name)
		return detailLoadedMsg{reqID: reqID, name: name, body: body, err: err}
	}
}

func editSeedCmd(ctx context.Context, c *cinc.Client, v Viewable, name string, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		body, err := v.Describe(ctx, c, name)
		return editSeedMsg{reqID: reqID, name: name, json: body, err: err}
	}
}

func saveCmd(ctx context.Context, c *cinc.Client, e Editable, name string, edited []byte) tea.Cmd {
	return func() tea.Msg {
		err := e.Save(ctx, c, name, edited)
		return mutationDoneMsg{verb: "Updated", name: name, err: err}
	}
}

func createCmd(ctx context.Context, c *cinc.Client, cr Creatable, doc []byte) tea.Cmd {
	return func() tea.Msg {
		res, err := cr.Create(ctx, c, doc)
		return mutationDoneMsg{verb: "Created", name: res.Name, secret: res.Secret, err: err}
	}
}

func createNamedCmd(ctx context.Context, c *cinc.Client, cr NamedCreatable, name string) tea.Cmd {
	return func() tea.Msg {
		res, err := cr.CreateNamed(ctx, c, name)
		return mutationDoneMsg{verb: "Created", name: res.Name, secret: res.Secret, err: err}
	}
}

func deleteCmd(ctx context.Context, c *cinc.Client, d Deletable, name string) tea.Cmd {
	return func() tea.Msg {
		err := d.Delete(ctx, c, name)
		return mutationDoneMsg{verb: "Deleted", name: name, err: err}
	}
}

func downloadCmd(ctx context.Context, c *cinc.Client, d Downloadable, name, dir string) tea.Cmd {
	return func() tea.Msg {
		path, err := d.Download(ctx, c, name, dir)
		return downloadDoneMsg{name: name, path: path, err: err}
	}
}

// serverInfoCmd makes one cheap authenticated request and reads the
// server's Chef API version out of the X-Ops-Server-Api-Version response
// header. A failure just leaves the version blank — it's title-bar trim,
// not load-bearing.
func serverInfoCmd(ctx context.Context, c *cinc.Client) tea.Cmd {
	return func() tea.Msg {
		_, resp, err := c.Nodes.List(ctx)
		if err != nil || resp == nil || resp.HTTPResponse == nil {
			return serverInfoMsg{}
		}
		return serverInfoMsg{version: parseAPIVersion(resp.HTTPResponse.Header.Get("X-Ops-Server-Api-Version"))}
	}
}

// parseAPIVersion turns a Chef X-Ops-Server-Api-Version header value
// (JSON like {"max_version":"2",…}) into a short label like "API v2".
func parseAPIVersion(header string) string {
	if header == "" {
		return ""
	}
	var v struct {
		MaxVersion string `json:"max_version"`
	}
	if err := json.Unmarshal([]byte(header), &v); err != nil || v.MaxVersion == "" {
		return ""
	}
	return "API v" + v.MaxVersion
}

// ----- tea.Model -------------------------------------------------------

func (m model) Init() tea.Cmd {
	// When a profile is resolved up front (single profile or --profile),
	// probe the server version immediately; the picker path fires it on
	// client-ready instead.
	if m.client != nil {
		return serverInfoCmd(m.ctx, m.client)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil
	case clientReadyMsg:
		return m.handleClientReady(msg)
	case listLoadedMsg:
		return m.handleListLoaded(msg), nil
	case detailLoadedMsg:
		return m.handleDetailLoaded(msg), nil
	case editSeedMsg:
		return m.handleEditSeed(msg)
	case mutationDoneMsg:
		return m.handleMutationDone(msg)
	case downloadDoneMsg:
		return m.handleDownloadDone(msg), nil
	case serverInfoMsg:
		m.serverVersion = msg.version
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	// Forward other messages (cursor blink, etc.) to whichever modal owns
	// a sub-component.
	return m.forwardToActive(msg)
}

func (m model) View() string {
	switch m.screen {
	case screenProfiles:
		return m.viewProfiles()
	case screenKinds:
		return m.viewKinds()
	case screenDetail:
		return m.viewDetail()
	case screenEditor:
		return m.editor.View()
	case screenConfirm:
		return m.viewConfirm()
	case screenName:
		return m.viewName()
	case screenResult:
		return m.viewResult()
	default:
		return m.viewList()
	}
}

// ----- resize ----------------------------------------------------------

func (m model) handleResize(msg tea.WindowSizeMsg) model {
	m.width, m.height = msg.Width, msg.Height
	// The detail viewport lives inside the bordered frame's body area, so
	// size it to the inner width and the body height frame() leaves it.
	vpW := max(1, msg.Width-2)
	vpH := m.bodyHeight()
	if !m.detailReady {
		m.detail = viewport.New(vpW, vpH)
		m.detailReady = true
	} else {
		m.detail.Width = vpW
		m.detail.Height = vpH
	}
	// The editor is only constructed when opened; resize it only while
	// it's the active screen to avoid touching a zero-value textarea.
	if m.screen == screenEditor {
		m.editor.SetSize(msg.Width, msg.Height)
	}
	return m
}

// forwardToActive routes non-key messages to the sub-component of the
// active modal screen.
func (m model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case screenEditor:
		m.editor, cmd = m.editor.Update(msg)
	case screenName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case screenList:
		if m.filtering {
			m.filter, cmd = m.filter.Update(msg)
		}
	case screenDetail:
		m.detail, cmd = m.detail.Update(msg)
	}
	return m, cmd
}

// ----- async message handlers ------------------------------------------

func (m model) handleClientReady(msg clientReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.listErr = msg.err.Error()
		m.screen = screenProfiles
		return m, nil
	}
	m.client = msg.client
	m.profileName = msg.name
	m.serverHost = hostFromProfile(m.opts, msg.name)
	m.serverVersion = ""
	m.listErr = ""
	m.screen = screenKinds
	return m, serverInfoCmd(m.ctx, m.client)
}

func (m model) handleListLoaded(msg listLoadedMsg) model {
	if msg.reqID != m.reqID {
		return m // stale
	}
	m.loading = false
	if msg.err != nil {
		m.listErr = msg.err.Error()
		m.rows = nil
		return m
	}
	m.listErr = ""
	m.rows = msg.rows
	if m.cursor >= len(m.filteredRows()) {
		m.cursor = max(0, len(m.filteredRows())-1)
	}
	return m
}

func (m model) handleDetailLoaded(msg detailLoadedMsg) model {
	if msg.reqID != m.reqID {
		return m
	}
	if msg.err != nil {
		m.detailErr = msg.err.Error()
		m.detail.SetContent("")
		return m
	}
	m.detailErr = ""
	m.detailName = msg.name
	m.detail.SetContent(msg.body)
	m.detail.GotoTop()
	return m
}

func (m model) handleEditSeed(msg editSeedMsg) (tea.Model, tea.Cmd) {
	if msg.reqID != m.reqID {
		return m, nil
	}
	if msg.err != nil {
		m.listErr = msg.err.Error()
		m.screen = screenList
		return m, nil
	}
	m.openEditor(actionEdit, msg.name, []byte(msg.json))
	return m, m.editor.Init()
}

func (m model) handleMutationDone(msg mutationDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.listErr = msg.err.Error()
		m.screen = screenList
		return m, nil
	}
	if msg.secret != "" {
		m.resultTitle = msg.verb + " " + msg.name
		m.resultBody = "One-time private key — copy it now, it will not be shown again:\n\n" + msg.secret
		m.screen = screenResult
		m.status = ""
		cmd := m.reloadList()
		return m, cmd
	}
	m.status = msg.verb + " " + msg.name
	m.screen = screenList
	cmd := m.reloadList()
	return m, cmd
}

func (m model) handleDownloadDone(msg downloadDoneMsg) model {
	if msg.err != nil {
		m.listErr = msg.err.Error()
		return m
	}
	m.status = "Downloaded " + msg.name + " to " + msg.path
	return m
}

// ----- shared list helpers ---------------------------------------------

// filteredRows returns the rows matching the active filter (case
// -insensitive substring on Name). With no filter it returns all rows.
func (m model) filteredRows() []Row {
	q := strings.TrimSpace(strings.ToLower(m.filter.Value()))
	if q == "" {
		return m.rows
	}
	out := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		if strings.Contains(strings.ToLower(r.Name), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m model) selectedRow() (Row, bool) {
	rows := m.filteredRows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return Row{}, false
	}
	return rows[m.cursor], true
}

// openList switches to a kind's list and fires the load. It does not
// touch the drill-down stack — callers manage that.
func (m *model) openList(k Kind) tea.Cmd {
	m.cur = k
	m.rows = nil
	m.cursor = 0
	m.loading = true
	m.listErr = ""
	m.status = ""
	m.filtering = false
	m.filter.SetValue("")
	m.screen = screenList
	return listCmd(m.ctx, m.client, k, m.nextReqID())
}

// reloadList re-fetches the current kind after a mutation.
func (m *model) reloadList() tea.Cmd {
	m.loading = true
	return listCmd(m.ctx, m.client, m.cur, m.nextReqID())
}

// openEditor seeds and shows the JSON editor.
func (m *model) openEditor(action editAction, name string, seed []byte) {
	m.editor = jsoneditor.New(seed, jsonSyntaxOnly)
	m.editor.SetSize(m.width, m.height)
	m.editKind = action
	m.editName = name
	m.screen = screenEditor
}

// jsonSyntaxOnly accepts any well-formed JSON; per-kind validation
// happens server-side on save. The editor still pretty-prints and
// previews before committing.
func jsonSyntaxOnly([]byte) error { return nil }
