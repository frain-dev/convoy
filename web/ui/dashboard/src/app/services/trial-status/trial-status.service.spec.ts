import {firstValueFrom} from 'rxjs';
import {take} from 'rxjs/operators';
import {TrialStatusService, TrialStatus} from './trial-status.service';

describe('TrialStatusService nav pill', () => {
	// Build the service with mocked http + org-state so refresh() exercises the real
	// pill logic without hitting the network.
	function makeService(subscriptionData: any, strategy = 'cloud') {
		const httpMock = {
			getOrganisation: () => ({ uid: 'org_1' }),
			request: (req: { url: string }) => {
				if (req.url.includes('/subscription') || req.url.includes('sh_subscription')) {
					return Promise.resolve({ data: subscriptionData });
				}
				// organisation endpoint (eligibility/offer)
				return Promise.resolve({ data: { trial_eligible: false, trial_offer: null } });
			}
		};
		const orgStateMock = { getBillingStrategy: () => Promise.resolve(strategy) };
		return new TrialStatusService(httpMock as any, orgStateMock as any);
	}

	async function statusAfterRefresh(service: TrialStatusService): Promise<TrialStatus | null> {
		await service.refresh();
		return firstValueFrom(service.status$.pipe(take(1)));
	}

	it('shows a static "Trial" pill while the org is on a trial (no countdown by design)', async () => {
		const service = makeService({ trial: true, trial_conversion_date: '2026-01-06T00:00:00Z' });
		const status = await statusAfterRefresh(service);

		expect(status?.label).toBe('Trial');
	});

	it('shows the pill even when the conversion date is missing', async () => {
		const service = makeService({ trial: true });
		const status = await statusAfterRefresh(service);

		expect(status?.label).toBe('Trial');
	});

	it('clears the pill when the org is not on a trial', async () => {
		const service = makeService({ trial: false });
		const status = await statusAfterRefresh(service);

		expect(status).toBeNull();
	});

	it('shows the pill for licensed self-hosted when the instance subscription is trialing', async () => {
		const service = makeService({ trial: true }, 'licensed_self_hosted');
		const status = await statusAfterRefresh(service);

		expect(status?.label).toBe('Trial');
	});

	it('clears the pill for oss billing strategy', async () => {
		const service = makeService({ trial: true }, 'oss');
		const status = await statusAfterRefresh(service);

		expect(status).toBeNull();
	});
});
