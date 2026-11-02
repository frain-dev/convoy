import { TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';
import { CreateEndpointComponent } from './create-endpoint.component';
import { CreatePortalEndpointComponent } from 'src/app/portal/create-portal-endpoint/create-portal-endpoint.component';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateEndpointService } from './create-endpoint.service';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { FilterService } from '../create-subscription/filter.service';
import { EndpointsService } from '../../pages/project/endpoints/endpoints.service';

describe('endpoint form URL validation', () => {
	beforeEach(() => TestBed.configureTestingModule({
		providers: [FormBuilder, ...[RbacService, GeneralService, CreateEndpointService, CreateSubscriptionService, FilterService, EndpointsService].map(provide => ({ provide, useValue: {} }))]
	}));
	for (const portal of [false, true]) {
		it(`${portal ? 'portal' : 'dashboard'} rejects a malformed paste in all URL controls without freezing`, () => {
			const form = TestBed.runInInjectionContext(() => {
				const route: any = { snapshot: { params: {}, queryParams: {} } };
				if (portal) return new CreatePortalEndpointComponent(route, {} as any, {} as any, {} as any).endpointForm;
				return new CreateEndpointComponent(new FormBuilder(), {} as any, {} as any, route, {} as any, {} as any, {} as any, {} as any, {} as any).addNewEndpointForm;
			});
			const valid = 'https://deploy-api.dev.example.com/deployment-service/v1/github/webhooks/deployment-protection';
			const start = performance.now();
			for (const field of ['url', 'slack_webhook_url', 'teams_webhook_url']) {
				const control = form.get(field)!;
				control.setValue(valid + ' ');
				expect(control.hasError('pattern')).withContext(field).toBe(true);
				control.setValue(valid);
				expect(control.valid).withContext(field).toBe(true);
			}
			expect(performance.now() - start).toBeLessThan(500);
		});
	}
});
