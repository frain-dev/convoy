import { Plan, PlanFeature } from './plan.service';

export function getFeaturesByCategory(
	plans: Plan[],
	category: 'core' | 'security' | 'support'
): PlanFeature[] {
	if (plans.length === 0) {
		return [];
	}

	const allFeatures = plans.flatMap(plan =>
		plan.features.filter(feature => feature.category === category)
	);

	return allFeatures.filter((feature, index, self) =>
		index === self.findIndex(item => item.name === feature.name)
	);
}

export function getFeatureValue(plans: Plan[], planId: string, featureName: string): string {
	const plan = plans.find(item => item.id === planId);
	if (!plan) {
		return 'Unsupported';
	}

	const feature = plan.features.find(item => item.name === featureName);
	return feature ? feature.value : 'Unsupported';
}

export function getFeatureValueType(
	plans: Plan[],
	planId: string,
	featureName: string
): 'supported' | 'unsupported' | 'plain' {
	const value = getFeatureValue(plans, planId, featureName);

	if (value === 'Supported') {
		return 'supported';
	}
	if (value === 'Unsupported') {
		return 'unsupported';
	}
	return 'plain';
}

export function comparisonTableGridColumns(planCount: number): string {
	if (planCount <= 0) {
		return '384px 1fr';
	}
	return `384px repeat(${planCount}, minmax(0, 1fr))`;
}

// Above this many bullets a collapsed card is assumed to overflow its fixed
// max-height, so the chevron (expand) affordance is shown. The collapsed list
// itself is clipped and faded in CSS rather than sliced, so nothing is hidden
// abruptly.
export const CARD_HIGHLIGHT_FADE_THRESHOLD = 4;

export interface CardHighlights {
	heading?: string;
	items: string[];
}

// Plan card bullets. Prefer curated marketing highlights (the "Everything in X,
// plus:" list); otherwise fall back to the supported feature rows so cards
// without curated copy still render something.
export function planCardHighlights(plan: Plan): CardHighlights {
	if (plan.highlights && plan.highlights.items.length > 0) {
		return { heading: plan.highlights.heading, items: plan.highlights.items };
	}

	const items = (plan.features || [])
		.filter(feature => (feature.value || '').toLowerCase() !== 'unsupported')
		.map(feature => feature.name);
	return { items };
}

export function hasCollapsedHighlights(highlights: CardHighlights): boolean {
	const rows = highlights.items.length + (highlights.heading ? 1 : 0);
	return rows > CARD_HIGHLIGHT_FADE_THRESHOLD;
}
