const CONVOY_CHECKOUT_PLAN_BASELINE = 'convoy_checkout_plan_baseline';

/** Stable identity for comparing subscription rows across GET /subscription responses. */
export function subscriptionPlanKey(sub: unknown): string {
  if (!sub || typeof sub !== 'object') {
    return '';
  }
  const o = sub as { plan?: { id?: string; name?: string }; plan_id?: string };
  const id = o.plan?.id ?? o.plan_id;
  if (id != null && String(id).trim() !== '') {
    return String(id);
  }
  const name = (o.plan?.name || '').trim().toLowerCase();
  return name ? `name:${name}` : '';
}

export function writeCheckoutPlanBaseline(orgId: string, planKey: string): void {
  if (!orgId) return;
  sessionStorage.setItem(
    CONVOY_CHECKOUT_PLAN_BASELINE,
    JSON.stringify({ orgId, planKey })
  );
}

export function readCheckoutPlanBaseline(orgId: string): { found: boolean; planKey: string } {
  try {
    const raw = sessionStorage.getItem(CONVOY_CHECKOUT_PLAN_BASELINE);
    if (!raw) {
      return { found: false, planKey: '' };
    }
    const o = JSON.parse(raw) as { orgId?: string; planKey?: unknown };
    if (o?.orgId !== orgId || typeof o.planKey !== 'string') {
      return { found: false, planKey: '' };
    }
    return { found: true, planKey: o.planKey };
  } catch {
    return { found: false, planKey: '' };
  }
}

export function clearCheckoutPlanBaseline(): void {
  sessionStorage.removeItem(CONVOY_CHECKOUT_PLAN_BASELINE);
}

/** Dedupes settings reloads for Maple return URLs that omit session_id. Cleared when checkout polling finishes or cleanup runs. */
export function checkoutProcessedNoSessionKey(orgId: string): string {
  return `checkout_processed_nosession_${orgId}`;
}

export function clearCheckoutProcessedNoSession(orgId: string): void {
  if (!orgId) return;
  localStorage.removeItem(checkoutProcessedNoSessionKey(orgId));
}
