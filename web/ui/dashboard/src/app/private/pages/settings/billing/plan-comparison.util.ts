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
