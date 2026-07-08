import {Plan} from './plan.service';

/** Canonical plan keys from the billing service catalog. */
export const PLAN_KEYS = {
	CLOUD_PRO: 'cloud_pro',
	CLOUD_PREMIUM: 'cloud_premium',
	CLOUD_ENTERPRISE: 'cloud_enterprise',
	SELF_HOSTED_PREMIUM: 'self_hosted_premium',
	SELF_HOSTED_ENTERPRISE: 'self_hosted_enterprise'
} as const;

const ENTERPRISE_KEYS = new Set<string>([
	PLAN_KEYS.CLOUD_ENTERPRISE,
	PLAN_KEYS.SELF_HOSTED_ENTERPRISE
]);

/** Legacy dashboard default-card ids before keys were aligned with billing. */
const LEGACY_ID_TO_KEY: Record<string, string> = {
	pro: PLAN_KEYS.CLOUD_PRO,
	enterprise: PLAN_KEYS.CLOUD_ENTERPRISE
};

/** Exact display names for older billing responses that omit plan.key. */
const LEGACY_NAME_TO_KEY: Record<string, string> = {
	pro: PLAN_KEYS.CLOUD_PRO,
	'cloud pro': PLAN_KEYS.CLOUD_PRO,
	'cloud premium': PLAN_KEYS.CLOUD_PREMIUM,
	enterprise: PLAN_KEYS.CLOUD_ENTERPRISE,
	'cloud enterprise': PLAN_KEYS.CLOUD_ENTERPRISE,
	premium: PLAN_KEYS.SELF_HOSTED_PREMIUM,
	'self-hosted premium': PLAN_KEYS.SELF_HOSTED_PREMIUM,
	'self-hosted enterprise': PLAN_KEYS.SELF_HOSTED_ENTERPRISE
};

const KNOWN_KEYS = new Set<string>(Object.values(PLAN_KEYS));

/**
 * Overwatch decides which real Maple-backed SKUs to offer and serves them under
 * their own catalog keys. The self-hosted "premium" SKU was deprecated; Business
 * is the premium-equivalent tier now, and the offered enterprise SKU is keyed
 * `enterprise`. The dashboard renders feature cards and checkout gating under the
 * canonical `self_hosted_premium` / `self_hosted_enterprise` keys, so alias the
 * served keys onto those. OW stays authoritative over which plans are offered;
 * this only maps their identity for display.
 */
const CATALOG_KEY_ALIAS: Record<string, string> = {
	business_annual: PLAN_KEYS.SELF_HOSTED_PREMIUM,
	business: PLAN_KEYS.SELF_HOSTED_PREMIUM,
	enterprise: PLAN_KEYS.SELF_HOSTED_ENTERPRISE
};

function aliasKey(key: string): string {
	return CATALOG_KEY_ALIAS[key] || key;
}

/** Resolve the billing catalog key for a plan row. Prefer plan.key; fall back for older binaries. */
export function resolvePlanKey(plan: Pick<Plan, 'key' | 'id' | 'name'>): string {
	const key = (plan.key || '').trim().toLowerCase();
	if (key) {
		return aliasKey(key);
	}

	const id = (plan.id || '').trim().toLowerCase();
	if (LEGACY_ID_TO_KEY[id]) {
		return LEGACY_ID_TO_KEY[id];
	}
	if (KNOWN_KEYS.has(id)) {
		return id;
	}

	const name = (plan.name || '').trim().toLowerCase();
	return LEGACY_NAME_TO_KEY[name] || '';
}

export function isEnterprisePlanKey(plan: Pick<Plan, 'key' | 'id' | 'name'>): boolean {
	return ENTERPRISE_KEYS.has(resolvePlanKey(plan));
}

export function isCheckoutCatalogPlanKey(plan: Pick<Plan, 'key' | 'id' | 'name'>): boolean {
	const key = resolvePlanKey(plan);
	return key === PLAN_KEYS.CLOUD_PRO || key === PLAN_KEYS.CLOUD_PREMIUM || key === PLAN_KEYS.CLOUD_ENTERPRISE;
}

/** Match API rows to default cards or checkout selections without substring name logic. */
export function plansMatch(
	left: Pick<Plan, 'key' | 'id' | 'name'>,
	right: Pick<Plan, 'key' | 'id' | 'name'>
): boolean {
	const leftKey = resolvePlanKey(left);
	const rightKey = resolvePlanKey(right);
	if (leftKey && rightKey) {
		return leftKey === rightKey;
	}
	if (left.id && right.id && left.id === right.id) {
		return true;
	}
	return left.name.trim().toLowerCase() === right.name.trim().toLowerCase();
}

export function trialPlanKeyForMode(mode: 'cloud' | 'self_hosted'): string {
	return mode === 'self_hosted' ? PLAN_KEYS.SELF_HOSTED_PREMIUM : PLAN_KEYS.CLOUD_PRO;
}
