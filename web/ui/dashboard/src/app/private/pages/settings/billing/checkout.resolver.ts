import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, ResolveFn } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';
import {
  checkoutProcessedNoSessionKey,
  readCheckoutPlanBaseline
} from './checkout-plan-baseline.util';

export interface CheckoutResolverData {
  needsPolling: boolean;
  checkoutProcessed: boolean;
  sessionId: string;
  orgId: string;
}

export const checkoutResolver: ResolveFn<CheckoutResolverData> = (route: ActivatedRouteSnapshot) => {
  const privateService = inject(PrivateService);

  const checkoutCompleted = route.queryParams?.['checkout'] === 'completed';
  const sessionId = route.queryParams?.['session_id'] || '';

  let orgId = privateService.getOrganisation?.uid || '';
  if (!orgId) {
    try {
      const rawOrg = localStorage.getItem('CONVOY_ORG');
      orgId = rawOrg ? JSON.parse(rawOrg)?.uid || '' : '';
    } catch (_) {
      orgId = '';
    }
  }

  if (!checkoutCompleted) {
    return { needsPolling: false, checkoutProcessed: false, sessionId: '', orgId: '' };
  }

  if (sessionId) {
    const checkoutKey = `checkout_processed_${sessionId}`;
    if (localStorage.getItem(checkoutKey)) {
      return { needsPolling: false, checkoutProcessed: true, sessionId, orgId: '' };
    }

    localStorage.setItem(checkoutKey, 'true');
    return { needsPolling: true, checkoutProcessed: false, sessionId, orgId };
  }

  // Maple upgrade/downgrade often returns without session_id; rely on pre-checkout baseline from the billing page.
  const baseline = readCheckoutPlanBaseline(orgId);
  if (!orgId || !baseline.found) {
    return { needsPolling: false, checkoutProcessed: false, sessionId: '', orgId: '' };
  }

  const noSessionKey = checkoutProcessedNoSessionKey(orgId);
  if (localStorage.getItem(noSessionKey)) {
    return { needsPolling: false, checkoutProcessed: true, sessionId: '', orgId: '' };
  }

  localStorage.setItem(noSessionKey, 'true');
  return { needsPolling: true, checkoutProcessed: false, sessionId: '', orgId };
};
