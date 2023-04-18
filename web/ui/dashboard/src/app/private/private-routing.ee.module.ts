import { inject, NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { PrivateComponent } from './private.component';
import { PrivateService } from './private.service';
import { routes } from './private-routers';

export const fetchOrganisations = async (privateService = inject(PrivateService)) => await privateService.getOrganizations();

routes[0].children?.push({
	path: 'ee',
	loadComponent: () => import('./pages/onboarding/onboarding.component').then(mod => mod.OnboardingComponent)
});

@NgModule({
	imports: [RouterModule.forChild(routes)],
	exports: [RouterModule]
})
export class PrivateRoutingModule {}
