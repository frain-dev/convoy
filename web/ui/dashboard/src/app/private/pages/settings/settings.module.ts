import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SettingsComponent } from './settings.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CardComponent } from 'src/app/components/card/card.component';
import { PageDirective } from 'src/app/components/page/page.component';
import { DeleteModalComponent } from '../../components/delete-modal/delete-modal.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { OrganisationSettingsComponent } from './organisation-settings/organisation-settings.component';
import { ConfigurationsComponent } from './configurations/configurations.component';
import { RouterModule, Routes } from '@angular/router';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { TeamsModule } from './teams/teams.module';
import { ConfigButtonComponent } from '../../components/config-button/config-button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';

const routes: Routes = [{ path: '', component: SettingsComponent }];

@NgModule({
	declarations: [SettingsComponent, OrganisationSettingsComponent, ConfigurationsComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		ReactiveFormsModule,
		CardComponent,
		PageDirective,
		DeleteModalComponent,

		SelectComponent,
		RadioComponent,
		ToggleComponent,
		ButtonComponent,
        ConfigButtonComponent,
		SkeletonLoaderComponent,
		TagComponent,
		CopyButtonComponent,
		DatePickerComponent,
		StatusColorModule,
		EmptyStateComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent,
		DialogDirective,
		TeamsModule,
        TooltipComponent
	]
})
export class SettingsModule {}
