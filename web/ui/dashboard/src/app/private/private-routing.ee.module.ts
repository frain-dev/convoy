import { inject, NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { PrivateService } from './private.service';
import { routes } from './private-routers';

// export const fetchOrganisations = async (privateService = inject(PrivateService)) => await privateService.getOrganisations();

routes[0].children?.push({
	path: 'ee',
	loadComponent: () => import('./pages/onboarding/onboarding.component').then(mod => mod.OnboardingComponent)
});

@NgModule({
	imports: [RouterModule.forChild(routes)],
	exports: [RouterModule]
})
export class PrivateRoutingModule {}
