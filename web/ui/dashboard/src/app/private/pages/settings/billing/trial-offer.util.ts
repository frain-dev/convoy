import {Plan} from './plan.service';

export interface TrialOfferLimit {
	key: string;
	label: string;
	value: number;
}

export type TrialDurationUnit = 'hour' | 'day';

// Shared trial terms for cloud org billing and self-hosted billing config.
export interface TrialOffer {
	duration_count?: number;
	duration_unit?: TrialDurationUnit | string;
	duration_days?: number;
	plan_name?: string;
	requires_card?: boolean;
	limits?: TrialOfferLimit[];
}

export type TrialBillingMode = 'cloud' | 'self_hosted';

/** Self-hosted billing config fields used by shared trial eligibility gates. */
export interface SelfHostedTrialConfig {
	license_configured?: boolean;
	active_checkout?: { attempt_id?: string } | null;
	trial_offer?: TrialOffer;
}

export interface SelfHostedTrialEligibility {
	billingStrategy: string;
	billingConfigLoaded: boolean;
	selfHostedConfig: SelfHostedTrialConfig | null | undefined;
	/** When true, hide the trial CTA while billing activation overlay is showing. */
	billingProvisioning?: boolean;
}

export function hasActiveSelfHostedCheckout(
	config: SelfHostedTrialConfig | null | undefined
): boolean {
	return !!config?.active_checkout?.attempt_id;
}

export function canStartSelfHostedTrial(input: SelfHostedTrialEligibility): boolean {
	return (
		input.billingStrategy === 'oss' &&
		input.billingConfigLoaded &&
		!input.selfHostedConfig?.license_configured &&
		!hasActiveSelfHostedCheckout(input.selfHostedConfig) &&
		!input.billingProvisioning
	);
}

export function resolveSelfHostedTrialOffer(
	config: SelfHostedTrialConfig | null | undefined
): TrialOffer | null {
	return config?.trial_offer ?? null;
}

export function resolveTrialModalMode(canStartSelfHosted: boolean): TrialBillingMode {
	return canStartSelfHosted ? 'self_hosted' : 'cloud';
}

/** Cloud empty state: no plan yet — block project create until trial or subscribe. */
export function canBlockProjectsWithCloudTrial(input: {
	isDisabled: boolean;
	canStartCloud: boolean;
}): boolean {
	if (input.isDisabled) {
		return false;
	}
	return input.canStartCloud;
}

/** OSS empty state: optional premium trial promo alongside Create Project. */
export function canShowSelfHostedTrialUpsell(
	input: SelfHostedTrialEligibility & { isDisabled?: boolean }
): boolean {
	if (input.isDisabled) {
		return false;
	}
	return canStartSelfHostedTrial(input);
}

/** Opens the trial modal from projects empty state (cloud gate or OSS upsell). */
export function canOfferProjectTrial(input: {
	isDisabled: boolean;
	canStartSelfHosted: boolean;
	canStartCloud: boolean;
}): boolean {
	if (input.isDisabled) {
		return false;
	}
	return input.canStartSelfHosted || input.canStartCloud;
}

const LIMIT_UNIT_ONE: Record<string, string> = {
	project_limit: 'Project',
	org_limit: 'Organization',
	user_limit: 'Team member',
	daily_event_limit: 'Event / day'
};

const LIMIT_UNIT_OTHER: Record<string, string> = {
	project_limit: 'Projects',
	org_limit: 'Organizations',
	user_limit: 'Team members',
	daily_event_limit: 'Events / day'
};

const EMPTY_TRIAL_OFFER: TrialOffer = {
	requires_card: false,
	limits: []
};

export function hasTrialLimits(offer: TrialOffer | null | undefined): boolean {
	return (offer?.limits?.length ?? 0) > 0;
}

/** Prefix before the inline "features" link in the trial modal. */
export function trialFeaturesLead(planName: string, offer: TrialOffer | null | undefined): string {
	const qualifier = hasTrialLimits(offer) ? 'All other supported' : 'All supported';
	return `${qualifier} ${planName}`;
}

export function trialFeaturesLine(planName: string, offer: TrialOffer): string {
	return `${trialFeaturesLead(planName, offer)} features are included.`;
}

export function formatTrialLimitLine(limit: TrialOfferLimit): string {
	const units = limit.value === 1 ? LIMIT_UNIT_ONE : LIMIT_UNIT_OTHER;
	const unit = units[limit.key] ?? limit.label;
	return `${limit.value} ${unit}`;
}

export function resolveTrialDuration(offer: TrialOffer): { count: number; unit: TrialDurationUnit } {
	if (offer.duration_count != null && offer.duration_unit) {
		const unit = offer.duration_unit.toString().toLowerCase() === 'hour' ? 'hour' : 'day';
		return { count: offer.duration_count, unit };
	}

	const days = offer.duration_days ?? 14;
	return { count: days, unit: 'day' };
}

export function formatTrialDuration(offer: TrialOffer): string {
	const { count, unit } = resolveTrialDuration(offer);
	if (unit === 'hour') {
		return count === 1 ? '1 hour' : `${count} hours`;
	}
	return count === 1 ? '1 day' : `${count} days`;
}

export function resolveTrialPlanName(
	mode: TrialBillingMode,
	offer: TrialOffer | null,
	catalogPlans: Plan[] = []
): string {
	if (offer?.plan_name) {
		return offer.plan_name;
	}

	const premiumPlan = catalogPlans.find(plan => {
		const key = (plan.key || '').toLowerCase();
		const name = plan.name.toLowerCase();
		if (mode === 'self_hosted') {
			return key.includes('premium') || name.includes('premium');
		}
		return (name.includes('pro') || key.includes('pro')) && !name.includes('enterprise');
	});

	if (premiumPlan?.name) {
		return premiumPlan.name;
	}

	return mode === 'self_hosted' ? 'Self-Hosted Premium' : 'Cloud Pro';
}

/** Trial features dialog: one SKU only (Premium / Cloud Pro), not the full checkout catalog. */
export function filterPlansToTrialPlan(plans: Plan[], trialPlanName: string): Plan[] {
	if (!trialPlanName || plans.length === 0) {
		return plans;
	}

	const target = trialPlanName.toLowerCase();
	const exact = plans.find(plan => plan.name === trialPlanName);
	if (exact) {
		return [exact];
	}

	const fuzzy = plans.find(plan => {
		const name = plan.name.toLowerCase();
		return name === target || name.includes(target) || target.includes(name);
	});
	if (fuzzy) {
		return [fuzzy];
	}

	const proTier = plans.find(plan => {
		const key = (plan.key || '').toLowerCase();
		const name = plan.name.toLowerCase();
		return (
			name.includes('premium') ||
			name.includes('pro') ||
			key.includes('premium') ||
			key.includes('pro')
		) && !name.includes('enterprise');
	});
	return proTier ? [proTier] : plans.slice(0, 1);
}

export function resolveTrialOffer(
	mode: TrialBillingMode,
	cloudOffer: TrialOffer | null,
	selfHostedOffer: TrialOffer | null
): TrialOffer {
	if (mode === 'self_hosted') {
		return selfHostedOffer ?? EMPTY_TRIAL_OFFER;
	}
	return cloudOffer ?? EMPTY_TRIAL_OFFER;
}

export function formatTrialIntro(
	mode: TrialBillingMode,
	cloudOffer: TrialOffer | null,
	selfHostedOffer: TrialOffer | null,
	catalogPlans: Plan[] = []
): string {
	const offer = resolveTrialOffer(mode, cloudOffer, selfHostedOffer);
	const planName = resolveTrialPlanName(mode, offer, catalogPlans);
	return `Try ${planName} free for ${formatTrialDuration(offer)}. No payment method required.`;
}

/** Projects empty state: one short line before the inline trial link (no duration). */
export function formatSelfHostedTrialUpsellLead(
	selfHostedOffer: TrialOffer | null,
	catalogPlans: Plan[] = []
): string {
	const planName = resolveTrialPlanName('self_hosted', selfHostedOffer, catalogPlans);
	return `Want ${planName} features?`;
}

export function formatCloudTrialUpsellLead(
	cloudOffer: TrialOffer | null,
	catalogPlans: Plan[] = []
): string {
	const planName = resolveTrialPlanName('cloud', cloudOffer, catalogPlans);
	return `Want ${planName} features?`;
}
