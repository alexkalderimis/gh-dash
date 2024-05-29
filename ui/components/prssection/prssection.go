package prssection

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dlvhdr/gh-dash/config"
	"github.com/dlvhdr/gh-dash/data"
	"github.com/dlvhdr/gh-dash/ui/components/pr"
	"github.com/dlvhdr/gh-dash/ui/components/section"
	"github.com/dlvhdr/gh-dash/ui/components/table"
	"github.com/dlvhdr/gh-dash/ui/constants"
	"github.com/dlvhdr/gh-dash/ui/context"
	"github.com/dlvhdr/gh-dash/ui/keys"
	"github.com/dlvhdr/gh-dash/utils"
)

const SectionType = "pr"

type Model struct {
	section.Model
	Prs []data.PullRequestData
}

func NewModel(
	id int,
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
	lastUpdated time.Time,
) Model {
	m := Model{}
	m.Model =
		section.NewModel(
			id,
			ctx,
			cfg.ToSectionConfig(),
			SectionType,
			GetSectionColumns(cfg, ctx),
			m.GetItemSingularForm(),
			m.GetItemPluralForm(),
			lastUpdated,
		)
	m.Prs = []data.PullRequestData{}

	return m
}

func (m Model) Update(msg tea.Msg) (section.Section, tea.Cmd) {
	var cmd tea.Cmd
	var err error

	switch msg := msg.(type) {

	case tea.KeyMsg:

		if m.IsSearchFocused() {
			switch {

			case msg.Type == tea.KeyCtrlC, msg.Type == tea.KeyEsc:
				m.SearchBar.SetValue(m.SearchValue)
				blinkCmd := m.SetIsSearching(false)
				return &m, blinkCmd

			case msg.Type == tea.KeyEnter:
				m.SearchValue = m.SearchBar.Value()
				m.SetIsSearching(false)
				m.ResetRows()
				return &m, tea.Batch(m.FetchNextPageSectionRows()...)
			}

			break
		}

		if m.IsPromptConfirmationFocused() {
			switch {

			case msg.Type == tea.KeyCtrlC, msg.Type == tea.KeyEsc:
				m.PromptConfirmationBox.Reset()
				cmd = m.SetIsPromptConfirmationShown(false)
				return &m, cmd

			case msg.Type == tea.KeyEnter:
				input := m.PromptConfirmationBox.Value()
				action := m.GetPromptConfirmationAction()
				if input == "Y" || input == "y" {
					switch action {
					case "close":
						cmd = m.close()
					case "reopen":
						cmd = m.reopen()
					case "ready":
						cmd = m.ready()
					case "merge":
						cmd = m.merge()
					}
				}

				m.PromptConfirmationBox.Reset()
				blinkCmd := m.SetIsPromptConfirmationShown(false)

				return &m, tea.Batch(cmd, blinkCmd)
			}

			break
		}

		switch {

		case key.Matches(msg, keys.PRKeys.Diff):
			cmd = m.diff()

		case key.Matches(msg, keys.PRKeys.Checkout):
			cmd, err = m.checkout()
			if err != nil {
				m.Ctx.Error = err
			}

		case key.Matches(msg, keys.PRKeys.WatchChecks):
			cmd = m.watchChecks()

		}

	case UpdatePRMsg:
		for i, currPr := range m.Prs {
			if currPr.Number == msg.PrNumber {
				if msg.IsClosed != nil {
					if *msg.IsClosed {
						currPr.State = "CLOSED"
					} else {
						currPr.State = "OPEN"
					}
				}
				if msg.NewComment != nil {
					currPr.Comments.Nodes = append(currPr.Comments.Nodes, *msg.NewComment)
				}
				if msg.AddedAssignees != nil {
					currPr.Assignees.Nodes = addAssignees(currPr.Assignees.Nodes, msg.AddedAssignees.Nodes)
				}
				if msg.RemovedAssignees != nil {
					currPr.Assignees.Nodes = removeAssignees(currPr.Assignees.Nodes, msg.RemovedAssignees.Nodes)
				}
				if msg.ReadyForReview != nil && *msg.ReadyForReview {
					currPr.IsDraft = false
				}
				if msg.IsMerged != nil && *msg.IsMerged {
					currPr.State = "MERGED"
					currPr.Mergeable = ""
				}
				m.Prs[i] = currPr
				m.Table.SetRows(m.BuildRows())
				break
			}
		}

	case SectionPullRequestsFetchedMsg:
		if m.LastFetchTaskId == msg.TaskId {
			if m.PageInfo != nil {
				m.Prs = append(m.Prs, msg.Prs...)
			} else {
				m.Prs = msg.Prs
			}
			m.TotalCount = msg.TotalCount
			m.PageInfo = &msg.PageInfo
			m.Table.SetRows(m.BuildRows())
			m.UpdateLastUpdated(time.Now())
			m.UpdateTotalItemsCount(m.TotalCount)
		}
	}

	search, searchCmd := m.SearchBar.Update(msg)
	m.Table.SetRows(m.BuildRows())
	m.SearchBar = search

	prompt, promptCmd := m.PromptConfirmationBox.Update(msg)
	m.PromptConfirmationBox = prompt
	return &m, tea.Batch(cmd, searchCmd, promptCmd)
}

func fromColumnConfig(config config.ColumnConfig) table.Column {
	return withColumnConfig(table.Column{}, config)
}

func withColumnConfig(column table.Column, config config.ColumnConfig) table.Column {
	column.Title = *config.Title
	column.Width = config.Width
	column.Hidden = config.Hidden

	return column
}

func GetSectionColumns(
	cfg config.PrsSectionConfig,
	ctx *context.ProgramContext,
) []table.Column {
	dLayout := ctx.Config.Defaults.Layout.Prs
	sLayout := cfg.Layout

	updatedAtLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.UpdatedAtIcon},
		dLayout.UpdatedAt,
		sLayout.UpdatedAt,
	)
	repoLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.RepoIcon},
		dLayout.Repo,
		sLayout.Repo,
	)
	titleLayout := config.MergeColumnConfigs(dLayout.Title, sLayout.Title)
	authorLayout := config.MergeColumnConfigs(dLayout.Author, sLayout.Author)
	assigneesLayout := config.MergeColumnConfigs(
		dLayout.Assignees,
		sLayout.Assignees,
	)
	baseLayout := config.MergeColumnConfigs(dLayout.Base, sLayout.Base)
	reviewStatusLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.ReviewIcon},
		dLayout.ReviewStatus,
		sLayout.ReviewStatus,
	)
	stateLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.StateIcon},
		dLayout.State,
		sLayout.State,
	)
	ciLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.CiIcon},
		dLayout.Ci,
		sLayout.Ci,
	)
	linesLayout := config.MergeColumnConfigs(
		config.ColumnConfig{Title: &ctx.Theme.Icons.DiffIcon},
		dLayout.Lines,
		sLayout.Lines,
	)

	return []table.Column{
		fromColumnConfig(updatedAtLayout),
		fromColumnConfig(stateLayout),
		fromColumnConfig(repoLayout),
		withColumnConfig(table.Column{Grow: utils.BoolPtr(true)}, titleLayout),
		fromColumnConfig(authorLayout),
		fromColumnConfig(assigneesLayout),
		fromColumnConfig(baseLayout),
		fromColumnConfig(reviewStatusLayout),
		withColumnConfig(table.Column{Width: &ctx.Styles.PrSection.CiCellWidth, Grow: new(bool)}, ciLayout),
		fromColumnConfig(linesLayout),
	}
}

func (m *Model) BuildRows() []table.Row {
	var rows []table.Row
	currItem := m.Table.GetCurrItem()
	for i, currPr := range m.Prs {
		i := i
		prModel := pr.PullRequest{Ctx: m.Ctx, Data: currPr}
		rows = append(
			rows,
			prModel.ToTableRow(currItem == i),
		)
	}

	if rows == nil {
		rows = []table.Row{}
	}

	return rows
}

func (m *Model) NumRows() int {
	return len(m.Prs)
}

type SectionPullRequestsFetchedMsg struct {
	Prs        []data.PullRequestData
	TotalCount int
	PageInfo   data.PageInfo
	TaskId     string
}

func (m *Model) GetCurrRow() data.RowData {
	if len(m.Prs) == 0 {
		return nil
	}
	pr := m.Prs[m.Table.GetCurrItem()]
	return &pr
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil {
		return nil
	}

	if m.PageInfo != nil && !m.PageInfo.HasNextPage {
		return nil
	}

	var cmds []tea.Cmd

	startCursor := time.Now().String()
	if m.PageInfo != nil {
		startCursor = m.PageInfo.StartCursor
	}
	taskId := fmt.Sprintf("fetching_prs_%d_%s", m.Id, startCursor)
	m.LastFetchTaskId = taskId
	task := context.Task{
		Id:        taskId,
		StartText: fmt.Sprintf(`Fetching PRs for "%s"`, m.Config.Title),
		FinishedText: fmt.Sprintf(
			`PRs for "%s" have been fetched`,
			m.Config.Title,
		),
		State: context.TaskStart,
		Error: nil,
	}
	startCmd := m.Ctx.StartTask(task)
	cmds = append(cmds, startCmd)

	fetchCmd := func() tea.Msg {
		limit := m.Config.Limit
		if limit == nil {
			limit = &m.Ctx.Config.Defaults.PrsLimit
		}
		res, err := data.FetchPullRequests(m.GetFilters(), *limit, m.PageInfo)
		if err != nil {
			return constants.TaskFinishedMsg{
				SectionId:   m.Id,
				SectionType: m.Type,
				TaskId:      taskId,
				Err:         err,
			}
		}

		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: m.Type,
			TaskId:      taskId,
			Msg: SectionPullRequestsFetchedMsg{
				Prs:        res.Prs,
				TotalCount: res.TotalCount,
				PageInfo:   res.PageInfo,
				TaskId:     taskId,
			},
		}
	}
	cmds = append(cmds, fetchCmd)

	return cmds
}

func (m *Model) ResetRows() {
	m.Prs = nil
	m.Table.Rows = nil
	m.ResetPageInfo()
	m.Table.ResetCurrItem()
}

func FetchAllSections(
	ctx context.ProgramContext,
) (sections []section.Section, fetchAllCmd tea.Cmd) {
	fetchPRsCmds := make([]tea.Cmd, 0, len(ctx.Config.PRSections))
	sections = make([]section.Section, 0, len(ctx.Config.PRSections))
	for i, sectionConfig := range ctx.Config.PRSections {
		sectionModel := NewModel(
			i+1,
			&ctx,
			sectionConfig,
			time.Now(),
		) // 0 is the search section
		sections = append(sections, &sectionModel)
		fetchPRsCmds = append(
			fetchPRsCmds,
			sectionModel.FetchNextPageSectionRows()...)
	}
	return sections, tea.Batch(fetchPRsCmds...)
}

func (m Model) GetItemSingularForm() string {
	return "PR"
}

func (m Model) GetItemPluralForm() string {
	return "PRs"
}

type UpdatePRMsg struct {
	PrNumber         int
	IsClosed         *bool
	NewComment       *data.Comment
	ReadyForReview   *bool
	IsMerged         *bool
	AddedAssignees   *data.Assignees
	RemovedAssignees *data.Assignees
}

func addAssignees(assignees, addedAssignees []data.Assignee) []data.Assignee {
	newAssignees := assignees
	for _, assignee := range addedAssignees {
		if !assigneesContains(newAssignees, assignee) {
			newAssignees = append(newAssignees, assignee)
		}
	}

	return newAssignees
}

func removeAssignees(
	assignees, removedAssignees []data.Assignee,
) []data.Assignee {
	newAssignees := []data.Assignee{}
	for _, assignee := range assignees {
		if !assigneesContains(removedAssignees, assignee) {
			newAssignees = append(newAssignees, assignee)
		}
	}

	return newAssignees
}

func assigneesContains(assignees []data.Assignee, assignee data.Assignee) bool {
	for _, a := range assignees {
		if assignee == a {
			return true
		}
	}
	return false
}
