import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';

import { PrivateRoutingModule } from './private-routing.module';
import { PrivateComponent } from './private.component';
import { CreateOrganisationModule } from './components/create-organisation/create-organisation.module';
import { AddAnalyticsModule } from './components/add-analytics/add-analytics.module';
import { ButtonComponent } from '../components/button/button.component';
import { BadgeComponent } from '../components/badge/badge.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';

@NgModule({
	declarations: [PrivateComponent],
	imports: [CommonModule, PrivateRoutingModule, CreateOrganisationModule, AddAnalyticsModule, DropdownComponent, ButtonComponent, BadgeComponent]
})
export class PrivateModule {}
