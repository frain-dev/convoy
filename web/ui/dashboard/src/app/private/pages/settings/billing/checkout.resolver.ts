import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, ResolveFn } from '@angular/router';
import { HttpService } from 'src/app/services/http/http.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from 'src/app/private/private.service';

export interface CheckoutResolverData {
  checkoutProcessed: boolean;
  subscriptionActive: boolean;
}

export const checkoutResolver: ResolveFn<CheckoutResolverData> = async (route: ActivatedRouteSnapshot) => {
  const httpService = inject(HttpService);
  const generalService = inject(GeneralService);
  const privateService = inject(PrivateService);

  const checkoutCompleted = route.queryParams?.['checkout'] === 'completed';
  const sessionId = route.queryParams?.['session_id'];

  if (!checkoutCompleted || !sessionId) {
    return { checkoutProcessed: false, subscriptionActive: false };
  }

  const checkoutKey = `checkout_processed_${sessionId}`;
  const alreadyProcessed = localStorage.getItem(checkoutKey);

  if (alreadyProcessed) {
    return { checkoutProcessed: true, subscriptionActive: false };
  }

  localStorage.setItem(checkoutKey, 'true');

  const orgId = privateService.getOrganisation?.uid || localStorage.getItem('CONVOY_ORG_ID') || '';
  
  generalService.showNotification({
    message: 'Verifying subscription status...',
    style: 'info'
  });

  const maxAttempts = 30;
  const pollInterval = 2000;
  let attempts = 0;
  let subscriptionActive = false;

  const poll = (): Promise<boolean> => {
    return new Promise((resolve) => {
      const doPoll = async () => {
        try {
          const response = await httpService.request({
            url: `/billing/organisations/${orgId}/subscription`,
            method: 'get',
            hideNotification: true
          });

          const subscription = response.data;
          if (subscription && subscription.status === 'active') {
            generalService.showNotification({
              message: 'Subscription activated successfully!',
              style: 'success'
            });
            subscriptionActive = true;
            resolve(true);
            return;
          }

          attempts++;
          if (attempts < maxAttempts) {
            setTimeout(doPoll, pollInterval);
          } else {
            generalService.showNotification({
              message: 'Subscription verification complete',
              style: 'info'
            });
            resolve(true);
          }
        } catch (error) {
          console.error('Failed to poll subscription status:', error);
          attempts++;
          if (attempts < maxAttempts) {
            setTimeout(doPoll, pollInterval);
          } else {
            generalService.showNotification({
              message: 'Unable to verify subscription. Please check billing page.',
              style: 'warning'
            });
            resolve(true);
          }
        }
      };

      setTimeout(doPoll, pollInterval);
    });
  };

  await poll();

  return { checkoutProcessed: true, subscriptionActive };
};
