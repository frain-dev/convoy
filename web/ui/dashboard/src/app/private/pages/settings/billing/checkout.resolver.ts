import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, ResolveFn } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';

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

  if (!checkoutCompleted || !sessionId) {
    return { needsPolling: false, checkoutProcessed: false, sessionId: '', orgId: '' };
  }

  const checkoutKey = `checkout_processed_${sessionId}`;
  if (localStorage.getItem(checkoutKey)) {
    return { needsPolling: false, checkoutProcessed: true, sessionId, orgId: '' };
  }

  localStorage.setItem(checkoutKey, 'true');
  const orgId = privateService.getOrganisation?.uid || localStorage.getItem('CONVOY_ORG_ID') || '';

  return { needsPolling: true, checkoutProcessed: false, sessionId, orgId };
};
