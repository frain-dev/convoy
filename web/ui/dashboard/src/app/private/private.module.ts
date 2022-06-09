import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';

import { PrivateRoutingModule } from './private-routing.module';
import { PrivateComponent } from './private.component';
import { CreateOrganisationModule } from './components/create-organisation/create-organisation.module';

@NgModule({
	declarations: [PrivateComponent],
	imports: [CommonModule, PrivateRoutingModule, CreateOrganisationModule]
})
export class PrivateModule {}
