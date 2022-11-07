import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SettingsComponent } from './settings.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CardComponent } from 'src/app/components/card/card.component';
import { PageComponent } from 'src/app/components/page/page.component';
import { DeleteModalComponent } from '../../components/delete-modal/delete-modal.component';
import { InputComponent } from 'src/app/components/input/input.component';
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
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';

const routes: Routes = [{ path: '', component: SettingsComponent }];

@NgModule({
	declarations: [SettingsComponent, OrganisationSettingsComponent, ConfigurationsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ReactiveFormsModule, CardComponent, PageComponent, DeleteModalComponent, InputComponent, SelectComponent, RadioComponent, ToggleComponent, ButtonComponent, SkeletonLoaderComponent, TagComponent, ModalComponent, CopyButtonComponent, DatePickerComponent, StatusColorModule, EmptyStateComponent]
})
export class SettingsModule {}
