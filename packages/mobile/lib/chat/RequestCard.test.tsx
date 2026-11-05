import { act, create, type ReactTestInstance, type ReactTestRenderer } from "react-test-renderer";
import { afterEach, expect, it, vi } from "vitest";

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true, IS_REACT_NATIVE_TEST_ENVIRONMENT: true });
const { keyboard } = vi.hoisted(() => ({ keyboard: { isVisible: false, height: 0 } }));
vi.mock("react-native", () => ({
	Platform: { OS: "ios" },
	ActivityIndicator: "ActivityIndicator", Pressable: "Pressable", ScrollView: "ScrollView", Text: "Text", TextInput: "TextInput", View: "View",
	StyleSheet: { create: (styles: unknown) => styles, hairlineWidth: 1 },
	useWindowDimensions: () => ({ width: 390, height: 844 }),
}));
vi.mock("react-native-gesture-handler", () => {
	const fling = () => ({ direction: () => fling(), onEnd: () => fling(), runOnJS: () => fling() });
	return {
		Directions: { LEFT: 1, RIGHT: 2 },
		Gesture: { Fling: fling, Race: () => ({}) },
		GestureDetector: ({ children }: { children: unknown }) => children,
	};
});
vi.mock("react-native-keyboard-controller", () => ({
	useKeyboardState: (select: (state: typeof keyboard) => unknown) => select(keyboard),
}));
vi.mock("react-native-reanimated", () => {
	const slide = { duration: () => slide };
	return { default: { View: "AnimatedView" }, SlideInLeft: slide, SlideInRight: slide, SlideOutLeft: slide, SlideOutRight: slide };
});
vi.mock("../icons", () => ({ Feather: "Feather" }));
vi.mock("../haptics", () => ({ haptics: { tap: vi.fn(), select: vi.fn(), success: vi.fn(), error: vi.fn() } }));
vi.mock("../ThemeProvider", () => ({ useTheme: () => ({}), useThemedStyles: (factory: (theme: object) => unknown) => factory({}) }));
vi.mock("../useReducedMotion", () => ({ useReducedMotion: () => true }));

import { RequestCard } from "./RequestCard";
import type { RequestDockModel } from "./requestDockModel";

const LONG_COMMAND = Array.from({ length: 40 }, (_, line) => `echo "step ${line}"`).join("; ");

const approval: RequestDockModel = {
	kind: "approval",
	title: "Approval needed",
	pages: [{ question: LONG_COMMAND, options: [{ id: "allow", label: "Allow" }, { id: "deny", label: "Deny" }], multi: false, required: true }],
	requestId: "req-1",
	sequence: 7,
	canAnswerInline: true,
	busy: false,
};

const question: RequestDockModel = {
	kind: "input",
	title: "Agent needs input",
	pages: [{
		question: "Which model?",
		options: [{ id: "a", label: "Layered", description: "A long description" }, { id: "b", label: "Flat" }],
		freeText: { name: "question_1_custom", placeholder: "Type your answer…", numeric: false },
		propertyName: "question_1",
		multi: false,
		required: true,
	}],
	requestId: "req-2",
	sequence: 9,
	canAnswerInline: true,
	busy: false,
};

let renderer: ReactTestRenderer;
afterEach(() => {
	if (renderer) act(() => renderer.unmount());
	keyboard.isVisible = false;
	keyboard.height = 0;
});

const render = (model: RequestDockModel) => {
	act(() => {
		renderer = create(<RequestCard model={model} onDecide={vi.fn(async () => {})} onResolveInput={vi.fn(async () => {})} onShow={vi.fn()} onDismiss={vi.fn()} />);
	});
	return renderer;
};

const insideScroll = (node: ReactTestInstance) => {
	for (let parent = node.parent; parent; parent = parent.parent) if ((parent.type as unknown) === "ScrollView") return true;
	return false;
};
const byLabel = (label: string) => renderer.root.findByProps({ accessibilityRole: "button", accessibilityLabel: label });
const cardMaxHeight = () => {
	const style = renderer.root.findAll((node) => (node.type as unknown) === "View")[0].props.style as object[];
	return Object.assign({}, ...style).maxHeight as number;
};

it("scrolls a long approval command while its decisions and close stay pinned outside it", () => {
	render(approval);
	expect(insideScroll(renderer.root.findByProps({ children: LONG_COMMAND }))).toBe(true);
	expect(insideScroll(byLabel("Allow"))).toBe(false);
	expect(insideScroll(byLabel("Deny"))).toBe(false);
	expect(insideScroll(byLabel("Dismiss and type instead"))).toBe(false);
});

it("caps the card at half the screen, and at half of what the keyboard leaves", () => {
	render(approval);
	expect(cardMaxHeight()).toBe(422);
	act(() => renderer.unmount());
	keyboard.isVisible = true;
	keyboard.height = 336;
	render(approval);
	expect(cardMaxHeight()).toBe(254);
});

it("scrolls a question's options with it and pins the typed answer", () => {
	render(question);
	expect(insideScroll(byLabel("Layered"))).toBe(true);
	expect(insideScroll(renderer.root.findByProps({ accessibilityLabel: "Type your answer…" }))).toBe(false);
});
