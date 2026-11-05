import {
	AlertTriangle,
	Check,
	CheckCircle2,
	ChevronRight,
	FileText,
	Play,
	Plus,
	RotateCcw,
	User,
} from "lucide-react";
import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Pressable } from "./ui/pressable";

export type PlanStatus = "draft" | "approved";

export type TaskStatus =
	| "not_assigned"
	| "queued"
	| "active"
	| "in_progress"
	| "completed"
	| "failed"
	| "blocked";

export interface PlanChecklistItem {
	id?: string;
	text: string;
	completed: boolean;
}

export interface PlanWorker {
	id?: string;
	name: string;
	status?: string;
	sessionId?: string;
	url?: string;
}

export interface PlanTask {
	id: string;
	title: string;
	description?: string;
	checklist?: PlanChecklistItem[];
	worker?: PlanWorker | null;
	status?: TaskStatus;
}

export interface PlanPhase {
	id?: string;
	number?: number | string;
	title: string;
	description?: string;
	tasks: PlanTask[];
}

export interface PlanSummary {
	id: string;
	title: string;
	status: PlanStatus;
	path: string;
	description?: string;
	updatedAt?: string;
}

export interface PlanDetail extends PlanSummary {
	phases: PlanPhase[];
}

export interface ProjectPlanOverviewProps {
	/** List of plan summaries */
	plans?: PlanSummary[];
	/** Currently loaded detail for the selected plan */
	selectedPlan?: PlanDetail | null;
	/** Currently selected plan ID */
	selectedPlanId?: string | null;
	/** Callback when a plan row is selected */
	onSelectPlan?: (planId: string) => void;
	/** Loading state for the plans list */
	isLoading?: boolean;
	/** Loading state for the selected plan detail */
	isDetailLoading?: boolean;
	/** Error state or message */
	error?: string | Error | null;
	/** Recovery action on error */
	onRetry?: () => void;
	/** Action to create a plan when none exist */
	onCreatePlan?: () => void;
	/** Callback to launch an eligible unassigned task with pending checklist */
	onLaunchTask?: (taskId: string, planId: string) => void;
	/** Optional callback when a worker session link is clicked */
	onOpenWorker?: (worker: PlanWorker, taskId: string) => void;
	/** Optional container class name */
	className?: string;
}

function resolveTaskPresentation(task: PlanTask): {
	label: string;
	variant: "neutral" | "outline" | "accent" | "success" | "warning" | "error";
	isActive: boolean;
	isCompleted: boolean;
} {
	const rawStatus = task.status;
	const workerStatus = task.worker?.status;
	const hasWorker = Boolean(task.worker && (task.worker.name || task.worker.id));
	const checklist = task.checklist ?? [];
	const allItemsComplete = checklist.length > 0 && checklist.every((i) => i.completed);

	if (rawStatus === "active" || rawStatus === "in_progress" || workerStatus === "running") {
		return { label: "Active", variant: "accent", isActive: true, isCompleted: false };
	}
	if (rawStatus === "completed" || workerStatus === "completed" || allItemsComplete) {
		return { label: "Completed", variant: "success", isActive: false, isCompleted: true };
	}
	if (rawStatus === "blocked") {
		return { label: "Blocked", variant: "error", isActive: false, isCompleted: false };
	}
	if (rawStatus === "failed" || workerStatus === "failed") {
		return { label: "Failed", variant: "error", isActive: false, isCompleted: false };
	}
	if (rawStatus === "queued") {
		return { label: "Queued", variant: "neutral", isActive: false, isCompleted: false };
	}
	if (!hasWorker || rawStatus === "not_assigned") {
		return { label: "Unassigned", variant: "outline", isActive: false, isCompleted: false };
	}

	return { label: "Pending", variant: "neutral", isActive: false, isCompleted: false };
}

export function ProjectPlanOverview({
	plans = [],
	selectedPlan,
	selectedPlanId,
	onSelectPlan,
	isLoading = false,
	isDetailLoading = false,
	error,
	onRetry,
	onCreatePlan,
	onLaunchTask,
	onOpenWorker,
	className,
}: ProjectPlanOverviewProps) {
	// Full loading state
	if (isLoading) {
		return (
			<div
				role="status"
				aria-label="Loading project plans"
				className={cn(
					"flex h-full min-h-[280px] w-full flex-col items-center justify-center p-6 text-center bg-background",
					className,
				)}
				data-testid="project-plan-loading"
			>
				<p className="text-sm text-muted-foreground">Loading project plans…</p>
			</div>
		);
	}

	// Error state with recovery
	if (error) {
		const errorMessage = typeof error === "string" ? error : error.message || "Failed to load plan";
		return (
			<div
				role="alert"
				className={cn(
					"flex h-full min-h-[280px] w-full flex-col items-center justify-center p-6 text-center bg-background",
					className,
				)}
				data-testid="project-plan-error"
			>
				<div className="flex size-9 items-center justify-center rounded-full bg-destructive/10 text-destructive mb-2.5">
					<AlertTriangle className="size-4" />
				</div>
				<h3 className="text-sm font-semibold text-foreground">Failed to load plan</h3>
				<p className="mt-1 max-w-md text-xs text-muted-foreground leading-normal">{errorMessage}</p>
				{onRetry && (
					<div className="mt-3">
						<Button variant="secondary" size="sm" onClick={onRetry}>
							<RotateCcw className="size-3.5 mr-1" />
							Retry
						</Button>
					</div>
				)}
			</div>
		);
	}

	// Empty state
	if (!plans || plans.length === 0) {
		return (
			<div
				className={cn(
					"flex h-full min-h-[280px] w-full flex-col items-center justify-center p-6 text-center bg-background",
					className,
				)}
				data-testid="project-plan-empty"
			>
				<div className="flex size-9 items-center justify-center rounded-full bg-secondary text-muted-foreground mb-2.5">
					<FileText className="size-4" />
				</div>
				<h3 className="text-sm font-semibold text-foreground">No project plans yet</h3>
				<p className="mt-1 max-w-sm text-xs text-muted-foreground leading-normal">
					Plans break down company goals into sequenced phases, assignable tasks, and verifiable checklists.
				</p>
				{onCreatePlan && (
					<div className="mt-3">
						<Button variant="primary" size="sm" onClick={onCreatePlan}>
							<Plus className="size-3.5 mr-1" />
							Create plan
						</Button>
					</div>
				)}
			</div>
		);
	}

	// Determine active plan ID
	const activePlanId = selectedPlanId ?? selectedPlan?.id ?? plans[0]?.id;
	const selectedSummary = plans.find((p) => p.id === activePlanId) ?? plans[0];

	// Detail must match activePlanId; otherwise treat as loading or waiting for detail fetch
	const hasMatchingDetail = Boolean(
		selectedPlan && selectedPlan.id === activePlanId && Array.isArray(selectedPlan.phases),
	);
	const showDetailLoading = isDetailLoading || (!hasMatchingDetail && Boolean(activePlanId));

	return (
		<div
			className={cn(
				"flex h-full w-full flex-col md:flex-row overflow-hidden bg-background text-foreground",
				className,
			)}
			data-testid="project-plan-overview"
		>
			{/* Left navigation rail: List of plans */}
			<nav
				aria-label="Project plans"
				className="flex flex-col border-b md:border-b-0 md:border-r border-border md:w-64 md:shrink-0 bg-background/50 overflow-y-auto"
				data-testid="project-plan-list"
			>
				<div className="sticky top-0 z-10 flex items-center justify-between border-b border-border bg-background/90 px-3 py-2">
					<span className="text-caption font-medium tracking-tight text-muted-foreground">
						Plans
					</span>
					<Badge variant="neutral" className="text-micro h-4 px-1.5">
						{plans.length}
					</Badge>
				</div>

				<div className="flex flex-col divide-y divide-border/50">
					{plans.map((plan) => {
						const isSelected = plan.id === activePlanId;
						const isApproved = plan.status === "approved";

						return (
							<Pressable
								key={plan.id}
								role="button"
								aria-current={isSelected ? "true" : undefined}
								onClick={() => onSelectPlan?.(plan.id)}
								className={cn(
									"group relative flex flex-col gap-0.5 px-3 py-2.5 text-left transition-colors",
									"hover:bg-interactive-hover focus-visible:bg-interactive-hover focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none",
									isSelected && "bg-secondary text-foreground",
								)}
								data-testid={`plan-item-${plan.id}`}
							>
								{isSelected && (
									<div className="absolute inset-y-0 left-0 w-0.5 bg-primary" />
								)}

								<div className="flex items-center justify-between gap-1.5 min-w-0">
									<span
										className={cn(
											"text-xs leading-snug truncate",
											isSelected ? "font-semibold text-foreground" : "text-foreground/90",
										)}
										title={plan.title}
									>
										{plan.title}
									</span>
									<Badge
										variant={isApproved ? "success" : "neutral"}
										className="text-micro shrink-0"
										data-testid={`plan-status-badge-${plan.id}`}
									>
										{isApproved ? "Approved" : "Draft"}
									</Badge>
								</div>

								<div className="flex items-center justify-between gap-1 min-w-0">
									<span className="font-mono text-2xs text-muted-foreground truncate" title={plan.path}>
										{plan.path}
									</span>
									<ChevronRight className="size-3 shrink-0 text-muted-foreground/60 transition-transform group-hover:translate-x-0.5" />
								</div>
							</Pressable>
						);
					})}
				</div>
			</nav>

			{/* Right pane: Selected plan detail view */}
			<main
				aria-label="Plan details"
				className="flex flex-1 flex-col min-w-0 overflow-y-auto"
				data-testid="project-plan-detail"
			>
				{selectedSummary ? (
					<div className="flex flex-col p-4 md:p-5 space-y-4">
						{/* Plan Header */}
						<header className="border-b border-border/60 pb-3">
							<div className="flex items-baseline justify-between gap-2">
								<h2
									className="text-sm font-semibold tracking-tight text-foreground break-words"
									data-testid="selected-plan-title"
								>
									{selectedSummary.title}
								</h2>
								<Badge
									variant={selectedSummary.status === "approved" ? "success" : "neutral"}
									className="shrink-0"
									data-testid="selected-plan-status"
								>
									{selectedSummary.status === "approved" ? "Approved" : "Draft"}
								</Badge>
							</div>

							<div className="mt-1 font-mono text-2xs text-muted-foreground truncate select-all">
								<span title={selectedSummary.path}>{selectedSummary.path}</span>
							</div>

							{selectedSummary.description && (
								<p className="mt-1.5 text-xs text-muted-foreground leading-normal">
									{selectedSummary.description}
								</p>
							)}
						</header>

						{/* Detail section: Loading detail vs Phases */}
						{showDetailLoading ? (
							<div
								role="status"
								aria-label="Loading plan details"
								className="flex h-36 flex-col items-center justify-center text-center text-muted-foreground text-xs"
								data-testid="plan-detail-loading"
							>
								Loading plan details…
							</div>
						) : selectedPlan && selectedPlan.phases && selectedPlan.phases.length > 0 ? (
							<div className="space-y-4" data-testid="plan-phases">
								{selectedPlan.phases.map((phase, idx) => (
									<section
										key={phase.id ?? `phase-${idx}`}
										aria-labelledby={`phase-title-${phase.id ?? idx}`}
										className="space-y-1.5"
										data-testid={`phase-${phase.id ?? idx}`}
									>
										<div className="flex items-baseline justify-between border-b border-border/40 pb-1">
											<h3
												id={`phase-title-${phase.id ?? idx}`}
												className="text-xs font-semibold text-foreground tracking-tight truncate"
											>
												{phase.number ? `Phase ${phase.number}: ` : ""}
												{phase.title}
											</h3>
											<span className="text-2xs text-muted-foreground shrink-0">
												{phase.tasks.length} {phase.tasks.length === 1 ? "task" : "tasks"}
											</span>
										</div>

										{phase.description && (
											<p className="text-2xs text-muted-foreground leading-normal pb-1">
												{phase.description}
											</p>
										)}

										{/* Tasks compact list */}
										<div className="divide-y divide-border/40 rounded-sm border border-border/60 bg-background/50">
											{phase.tasks.map((task) => (
												<TaskRow
													key={task.id}
													task={task}
													planId={selectedPlan.id}
													isPlanApproved={selectedPlan.status === "approved"}
													onLaunchTask={onLaunchTask}
													onOpenWorker={onOpenWorker}
												/>
											))}
										</div>
									</section>
								))}
							</div>
						) : (
							<div
								className="rounded-sm border border-dashed border-border/60 p-4 text-center text-xs text-muted-foreground"
								data-testid="no-tasks-message"
							>
								No phases defined in this plan.
							</div>
						)}
					</div>
				) : (
					<div
						className="flex h-full min-h-[280px] flex-col items-center justify-center p-6 text-center text-muted-foreground text-xs"
						data-testid="no-plan-selected"
					>
						Select a plan to view its phases and tasks.
					</div>
				)}
			</main>
		</div>
	);
}

interface TaskRowProps {
	task: PlanTask;
	planId: string;
	isPlanApproved: boolean;
	onLaunchTask?: (taskId: string, planId: string) => void;
	onOpenWorker?: (worker: PlanWorker, taskId: string) => void;
}

function TaskRow({
	task,
	planId,
	isPlanApproved,
	onLaunchTask,
	onOpenWorker,
}: TaskRowProps) {
	const presentation = resolveTaskPresentation(task);
	const checklist = task.checklist ?? [];
	const hasItems = checklist.length > 0;
	const allItemsComplete = hasItems && checklist.every((i) => i.completed);

	const hasWorker = Boolean(task.worker && (task.worker.name || task.worker.id));
	const isBlocked = task.status === "blocked";
	const isStatusCompleted = task.status === "completed";
	const isCompleted = isStatusCompleted || allItemsComplete;
	const isActive = task.status === "active" || task.status === "in_progress";
	const isQueued = task.status === "queued";
	const isFailedWithWorker = (task.status === "failed" || task.worker?.status === "failed") && hasWorker;
	const hasCallback = typeof onLaunchTask === "function";

	// Launch condition:
	// - Plan approved
	// - Pending checklist (items exist and at least one incomplete)
	// - No worker
	// - onLaunchTask provided
	// - Not queued, active, blocked, completed, failed-with-worker
	const canLaunch =
		isPlanApproved &&
		hasCallback &&
		!hasWorker &&
		hasItems &&
		!allItemsComplete &&
		!isBlocked &&
		!isCompleted &&
		!isActive &&
		!isQueued &&
		!isFailedWithWorker;

	// Deterministic visible reason when disabled
	let disabledReason: string | null = null;
	if (!canLaunch) {
		if (!hasCallback) {
			disabledReason = "Task launch not configured";
		} else if (hasWorker) {
			disabledReason = "Worker already assigned";
		} else if (isActive) {
			disabledReason = "Task is currently active";
		} else if (isStatusCompleted) {
			disabledReason = "Task is already complete";
		} else if (allItemsComplete) {
			disabledReason = "All checklist items complete";
		} else if (isBlocked) {
			disabledReason = "Task is blocked";
		} else if (isQueued) {
			disabledReason = "Task is already queued";
		} else if (isFailedWithWorker) {
			disabledReason = "Worker assigned to failed task";
		} else if (!isPlanApproved && !hasItems) {
			disabledReason = "Requires approved plan and checklist items";
		} else if (!isPlanApproved) {
			disabledReason = "Requires approved plan";
		} else if (!hasItems) {
			disabledReason = "Checklist required to launch";
		} else {
			disabledReason = "Task cannot be launched";
		}
	}

	return (
		<div
			className={cn(
				"flex flex-col gap-1.5 p-2.5 transition-colors text-xs",
				presentation.isActive && "bg-status-working/5",
			)}
			data-testid={`task-card-${task.id}`}
		>
			{/* Top row: Title + Status Badge */}
			<div className="flex items-start justify-between gap-2 min-w-0">
				<div className="min-w-0 flex-1">
					<div className="flex items-center gap-1.5">
						{presentation.isCompleted && (
							<CheckCircle2 className="size-3 shrink-0 text-status-ready" />
						)}
						<span
							className={cn(
								"font-medium text-foreground break-words leading-tight",
								presentation.isCompleted && "line-through text-muted-foreground",
							)}
						>
							{task.title}
						</span>
					</div>

					{task.description && (
						<p className="mt-0.5 text-2xs text-muted-foreground leading-normal">
							{task.description}
						</p>
					)}
				</div>

				<Badge
					variant={presentation.variant}
					className="text-micro shrink-0"
					data-testid={`task-status-${task.id}`}
				>
					{presentation.label}
				</Badge>
			</div>

			{/* Worker metadata */}
			<div className="flex flex-wrap items-center gap-1.5 text-2xs text-muted-foreground">
				{task.worker ? (
					<div className="flex items-center gap-1" data-testid={`task-worker-${task.id}`}>
						<User className="size-3 text-muted-foreground shrink-0" />
						<span className="font-medium text-foreground">{task.worker.name}</span>
						{task.worker.status && (
							<span
								className={cn(
									"font-mono capitalize",
									task.worker.status === "running" && "text-status-working",
									task.worker.status === "completed" && "text-status-ready",
									task.worker.status === "failed" && "text-destructive",
								)}
								data-testid={`worker-status-${task.id}`}
							>
								({task.worker.status})
							</span>
						)}

						{/* Only render link if handler is provided and session/url exists */}
						{onOpenWorker && (task.worker.sessionId || task.worker.url) && (
							<Button
								variant="link"
								size="xs"
								className="h-auto p-0 text-2xs text-primary ml-1"
								onClick={() => onOpenWorker(task.worker!, task.id)}
								data-testid={`worker-link-${task.id}`}
							>
								Session
							</Button>
						)}
					</div>
				) : (
					<span className="text-muted-foreground/75" data-testid={`task-unassigned-${task.id}`}>
						No worker assigned
					</span>
				)}
			</div>

			{/* Checklist items */}
			{hasItems && (
				<div className="space-y-1 pt-0.5" data-testid={`task-checklist-section-${task.id}`}>
					<div className="space-y-0.5 rounded-xs bg-secondary/30 p-1.5" data-testid={`task-checklist-items-${task.id}`}>
						{checklist.map((item, i) => (
							<div
								key={item.id ?? `cl-${i}`}
								className="flex items-center gap-1.5 text-2xs"
								data-testid={`checklist-item-${item.id ?? i}`}
							>
								<span
									className={cn(
										"flex size-3 shrink-0 items-center justify-center rounded-xs border",
										item.completed
											? "border-status-ready bg-status-ready/20 text-status-ready"
											: "border-border text-transparent",
									)}
									aria-hidden="true"
								>
									{item.completed && <Check className="size-2 stroke-[3]" />}
								</span>
								<span
									className={cn(
										"min-w-0 flex-1 truncate",
										item.completed ? "text-muted-foreground line-through" : "text-foreground",
									)}
								>
									{item.text}
								</span>
							</div>
						))}
					</div>
				</div>
			)}

			{/* Launch button & visible disabled reason */}
			<div className="flex flex-wrap items-center gap-2 pt-1">
				<Button
					size="xs"
					variant={canLaunch ? "primary" : "secondary"}
					disabled={!canLaunch}
					onClick={() => onLaunchTask?.(task.id, planId)}
					aria-describedby={disabledReason ? `disabled-reason-${task.id}` : undefined}
					data-testid={`launch-task-btn-${task.id}`}
				>
					<Play className="size-2.5 mr-1" />
					Launch task
				</Button>

				{disabledReason && (
					<span
						id={`disabled-reason-${task.id}`}
						className="text-2xs text-muted-foreground"
						data-testid={`launch-disabled-reason-${task.id}`}
					>
						{disabledReason}
					</span>
				)}
			</div>
		</div>
	);
}
