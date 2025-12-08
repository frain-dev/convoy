import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { AdminComponent } from './admin.component';
import { FeatureFlagsComponent } from './feature-flags/feature-flags.component';
import { OrganisationOverridesComponent } from './organisation-overrides/organisation-overrides.component';
import { CircuitBreakerConfigComponent } from './circuit-breaker-config/circuit-breaker-config.component';
import { ResendEventsComponent } from './resend-events/resend-events.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from '../../components/loader/loader.module';
import { LabelComponent, InputFieldDirective, InputErrorComponent, InputDirective } from 'src/app/components/input/input.component';
import { CardComponent } from 'src/app/components/card/card.component';

const routes: Routes = [{ path: '', component: AdminComponent }];

@NgModule({
	declarations: [
		AdminComponent,
		FeatureFlagsComponent,
		OrganisationOverridesComponent,
		CircuitBreakerConfigComponent,
		ResendEventsComponent
	],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		ReactiveFormsModule,
		CardComponent,
		SelectComponent,
		ToggleComponent,
		TagComponent,
		ButtonComponent,
		LoaderModule,
		LabelComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective
	]
})
export class AdminModule {}
