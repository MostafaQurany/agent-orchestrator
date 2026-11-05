import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
	ProjectPlanOverview,
	type PlanDetail,
	type PlanSummary,
} from "./ProjectPlanOverview";

describe("ProjectPlanOverview", () => {
	const samplePlans: PlanSummary[] = [
		{
			id: "plan-1",
			title: "MVP Launch Plan",
			status: "approved",
			path: "plans/company/mvp.json",
			description: "Foundational company MVP roadmap",
		},
		{
			id: "plan-2",
			title: "Billing & Subscriptions Draft",
			status: "draft",
			path: "plans/company/billing.json",
			description: "Work in progress draft for payments",
		},
	];

	const sampleDetail: PlanDetail = {
		id: "plan-1",
		title: "MVP Launch Plan",
		status: "approved",
		path: "plans/company/mvp.json",
		description: "Foundational company MVP roadmap",
		phases: [
			{
				id: "phase-1",
				number: 1,
				title: "Infrastructure Setup",
				description: "Set up daemon and database",
				tasks: [
					{
						id: "task-eligible",
						title: "Provision SQLite Store",
						checklist: [
							{ text: "Write migration", completed: true },
							{ text: "Generate sqlc models", completed: false },
						],
					},
					{
						id: "task-active",
						title: "Active Working Task",
						status: "active",
						worker: {
							id: "worker-codex",
							name: "Codex Worker",
							status: "running",
							sessionId: "session-123",
						},
						checklist: [{ text: "In progress item", completed: false }],
					},
					{
						id: "task-completed",
						title: "Completed Task",
						status: "completed",
						worker: {
							name: "Claude Worker",
							status: "completed",
						},
						checklist: [
							{ text: "Task item 1", completed: true },
							{ text: "Task item 2", completed: true },
						],
					},
					{
						id: "task-assigned-idle",
						title: "Assigned Idle Task",
						worker: {
							name: "Assigned Worker",
							status: "idle",
						},
						checklist: [{ text: "Pending item", completed: false }],
					},
					{
						id: "task-no-checklist",
						title: "Unassigned Without Checklist",
						checklist: [],
					},
				],
			},
		],
	};

	it("renders loading state with accessible label", () => {
		render(<ProjectPlanOverview isLoading />);
		expect(screen.getByRole("status")).toHaveAccessibleName("Loading project plans");
		expect(screen.getByText("Loading project plans…")).toBeInTheDocument();
	});

	it("renders error state with recovery callback", async () => {
		const user = userEvent.setup();
		const handleRetry = vi.fn();

		render(
			<ProjectPlanOverview
				error="Failed to parse plan file"
				onRetry={handleRetry}
			/>,
		);

		expect(screen.getByRole("alert")).toBeInTheDocument();
		expect(screen.getByText("Failed to parse plan file")).toBeInTheDocument();

		const retryBtn = screen.getByRole("button", { name: /retry/i });
		await user.click(retryBtn);
		expect(handleRetry).toHaveBeenCalledTimes(1);
	});

	it("renders empty state with plan creation guidance and callback", async () => {
		const user = userEvent.setup();
		const handleCreate = vi.fn();

		render(<ProjectPlanOverview plans={[]} onCreatePlan={handleCreate} />);

		expect(screen.getByTestId("project-plan-empty")).toBeInTheDocument();
		expect(screen.getByText("No project plans yet")).toBeInTheDocument();

		const createBtn = screen.getByRole("button", { name: /create plan/i });
		await user.click(createBtn);
		expect(handleCreate).toHaveBeenCalledTimes(1);
	});

	it("renders plans list with selection semantics, keyboard focus, and callback", async () => {
		const user = userEvent.setup();
		const handleSelect = vi.fn();

		render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
				onSelectPlan={handleSelect}
			/>,
		);

		const planList = screen.getByTestId("project-plan-list");
		const firstItem = within(planList).getByTestId("plan-item-plan-1");
		const secondItem = within(planList).getByTestId("plan-item-plan-2");

		expect(firstItem).toHaveAttribute("aria-current", "true");
		expect(secondItem).not.toHaveAttribute("aria-current");

		await user.click(secondItem);
		expect(handleSelect).toHaveBeenCalledWith("plan-2");
	});

	it("shows detail loading state when selected summary is not yet loaded into full detail", () => {
		render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={null}
				selectedPlanId="plan-2"
			/>,
		);

		expect(screen.getByTestId("plan-detail-loading")).toHaveTextContent(
			"Loading plan details…",
		);
	});

	it("distinguishes draft versus approved plan status badges in list and detail header", () => {
		const { rerender } = render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
			/>,
		);

		expect(screen.getByTestId("selected-plan-status")).toHaveTextContent("Approved");

		const draftDetail: PlanDetail = {
			...sampleDetail,
			id: "plan-2",
			title: "Billing & Subscriptions Draft",
			status: "draft",
		};

		rerender(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={draftDetail}
				selectedPlanId="plan-2"
			/>,
		);

		expect(screen.getByTestId("selected-plan-status")).toHaveTextContent("Draft");
	});

	it("enables task launch ONLY for approved plan, pending checklist, unassigned worker, with callback", async () => {
		const user = userEvent.setup();
		const handleLaunch = vi.fn();

		render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
				onLaunchTask={handleLaunch}
			/>,
		);

		// task-eligible is approved, unassigned, has pending checklist -> enabled
		const eligibleBtn = screen.getByTestId("launch-task-btn-task-eligible");
		expect(eligibleBtn).not.toBeDisabled();
		expect(
			screen.queryByTestId("launch-disabled-reason-task-eligible"),
		).not.toBeInTheDocument();

		await user.click(eligibleBtn);
		expect(handleLaunch).toHaveBeenCalledWith("task-eligible", "plan-1");
	});

	it("disables task launch with visible reasons for active, completed, assigned, queued, or blocked tasks", () => {
		const complexDetail: PlanDetail = {
			id: "plan-1",
			title: "Approved Plan",
			status: "approved",
			path: "plans/plan.json",
			phases: [
				{
					title: "Phase 1",
					tasks: [
						{
							id: "t-active",
							title: "Active Task",
							status: "active",
							checklist: [{ text: "Item", completed: false }],
						},
						{
							id: "t-completed",
							title: "Completed Task",
							status: "completed",
							checklist: [{ text: "Item", completed: true }],
						},
						{
							id: "t-assigned",
							title: "Assigned Task",
							worker: { name: "Agent Worker" },
							checklist: [{ text: "Item", completed: false }],
						},
						{
							id: "t-queued",
							title: "Queued Task",
							status: "queued",
							checklist: [{ text: "Item", completed: false }],
						},
						{
							id: "t-blocked",
							title: "Blocked Task",
							status: "blocked",
							checklist: [{ text: "Item", completed: false }],
						},
						{
							id: "t-all-done",
							title: "All Complete Checklist",
							checklist: [{ text: "Done item", completed: true }],
						},
						{
							id: "t-no-cl",
							title: "No Checklist",
							checklist: [],
						},
					],
				},
			],
		};

		render(
			<ProjectPlanOverview
				plans={[{ id: "plan-1", title: "Approved Plan", status: "approved", path: "p" }]}
				selectedPlan={complexDetail}
				selectedPlanId="plan-1"
				onLaunchTask={vi.fn()}
			/>,
		);

		// Active task
		expect(screen.getByTestId("launch-task-btn-t-active")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-active")).toHaveTextContent(
			"Task is currently active",
		);

		// Completed task
		expect(screen.getByTestId("launch-task-btn-t-completed")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-completed")).toHaveTextContent(
			"Task is already complete",
		);

		// Assigned task
		expect(screen.getByTestId("launch-task-btn-t-assigned")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-assigned")).toHaveTextContent(
			"Worker already assigned",
		);

		// Queued task
		expect(screen.getByTestId("launch-task-btn-t-queued")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-queued")).toHaveTextContent(
			"Task is already queued",
		);

		// Blocked task
		expect(screen.getByTestId("launch-task-btn-t-blocked")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-blocked")).toHaveTextContent(
			"Task is blocked",
		);

		// All checklist complete
		expect(screen.getByTestId("launch-task-btn-t-all-done")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-all-done")).toHaveTextContent(
			"All checklist items complete",
		);

		// No checklist
		expect(screen.getByTestId("launch-task-btn-t-no-cl")).toBeDisabled();
		expect(screen.getByTestId("launch-disabled-reason-t-no-cl")).toHaveTextContent(
			"Checklist required to launch",
		);
	});

	it("disables task launch when plan is in draft status", () => {
		const draftDetail: PlanDetail = {
			...sampleDetail,
			id: "plan-draft",
			status: "draft",
		};

		render(
			<ProjectPlanOverview
				plans={[{ id: "plan-draft", title: "Draft", status: "draft", path: "p" }]}
				selectedPlan={draftDetail}
				selectedPlanId="plan-draft"
				onLaunchTask={vi.fn()}
			/>,
		);

		expect(screen.getByTestId("launch-task-btn-task-eligible")).toBeDisabled();
		expect(
			screen.getByTestId("launch-disabled-reason-task-eligible"),
		).toHaveTextContent("Requires approved plan");
	});

	it("disables task launch when onLaunchTask callback is not provided", () => {
		render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
			/>,
		);

		expect(screen.getByTestId("launch-task-btn-task-eligible")).toBeDisabled();
		expect(
			screen.getByTestId("launch-disabled-reason-task-eligible"),
		).toHaveTextContent("Task launch not configured");
	});

	it("renders session link only when onOpenWorker handler is provided", async () => {
		const user = userEvent.setup();
		const handleOpenWorker = vi.fn();

		const { rerender } = render(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
				onOpenWorker={handleOpenWorker}
			/>,
		);

		const link = screen.getByTestId("worker-link-task-active");
		expect(link).toBeInTheDocument();
		await user.click(link);
		expect(handleOpenWorker).toHaveBeenCalledWith(
			expect.objectContaining({ name: "Codex Worker" }),
			"task-active",
		);

		// Rerender without handler -> no link
		rerender(
			<ProjectPlanOverview
				plans={samplePlans}
				selectedPlan={sampleDetail}
				selectedPlanId="plan-1"
				onOpenWorker={undefined}
			/>,
		);

		expect(screen.queryByTestId("worker-link-task-active")).not.toBeInTheDocument();
		expect(screen.getByTestId("task-worker-task-active")).toHaveTextContent(
			"Codex Worker",
		);
	});

	it("gracefully renders long titles and paths without layout crash", () => {
		const longDetail: PlanDetail = {
			id: "plan-long",
			title: "A Very Long Plan Title Testing Wrapping In Constrained Views",
			status: "approved",
			path: "nested/path/to/enterprise/project/specification/v2/plan.json",
			phases: [
				{
					title: "Phase Long Title",
					tasks: [
						{
							id: "task-long",
							title: "Very Long Task Title To Validate Text Wrap",
							checklist: [{ text: "Checklist detail text", completed: false }],
						},
					],
				},
			],
		};

		render(
			<ProjectPlanOverview
				plans={[
					{
						id: "plan-long",
						title: longDetail.title,
						status: longDetail.status,
						path: longDetail.path,
					},
				]}
				selectedPlan={longDetail}
				selectedPlanId="plan-long"
			/>,
		);

		expect(screen.getByTestId("project-plan-overview")).toBeInTheDocument();
		expect(screen.getByTestId("selected-plan-title")).toHaveTextContent(longDetail.title);
	});
});
