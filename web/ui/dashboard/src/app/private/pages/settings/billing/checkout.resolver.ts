import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, ResolveFn } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';

export interface CheckoutResolverData {
  needsPolling: boolean;
  checkoutProcessed: boolean;
  sessionId: string;
  orgId: string;
  token: string;
  attemptId: string;
}

export const checkoutResolver: ResolveFn<CheckoutResolverData> = (route: ActivatedRouteSnapshot) => {
  const privateService = inject(PrivateService);

  const checkoutCompleted = route.queryParams?.['checkout'] === 'completed';
  const token = route.queryParams?.['token'] || '';
  const attemptId = route.queryParams?.['attempt_id'] || '';
  const sessionId = route.queryParams?.['session_id'] || '';

  if (!checkoutCompleted || (!token && !sessionId && !attemptId)) {
    return { needsPolling: false, checkoutProcessed: false, sessionId: '', orgId: '', token: '', attemptId: '' };
  }

  const checkoutKey = attemptId || sessionId;
  if (checkoutKey && localStorage.getItem(`checkout_processed_${checkoutKey}`)) {
    return { needsPolling: false, checkoutProcessed: true, sessionId, orgId: '', token, attemptId };
  }

  const orgId = privateService.getOrganisation?.uid || '';

  return { needsPolling: true, checkoutProcessed: false, sessionId, orgId, token, attemptId };
};
