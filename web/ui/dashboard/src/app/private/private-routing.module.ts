import { inject, NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { PrivateService } from './private.service';
import { routes } from './private-routers';

export const fetchOrganisations = async (privateService = inject(PrivateService)) => await privateService.getOrganizations();

@NgModule({
	imports: [RouterModule.forChild(routes)],
	exports: [RouterModule]
})
export class PrivateRoutingModule {}
