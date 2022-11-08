import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AccountComponent } from './account.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { PageComponent } from 'src/app/components/page/page.component';
import { InputComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { PersonalSettingsComponent } from './personal-settings/personal-settings.component';
import { ProfileSettingsComponent } from './profile-settings/profile-settings.component';
import { SecuritySettingsComponent } from './security-settings/security-settings.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DeleteModalComponent } from '../../components/delete-modal/delete-modal.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { SelectComponent } from 'src/app/components/select/select.component';

const routes: Routes = [{ path: '', component: AccountComponent }];

@NgModule({
	declarations: [AccountComponent, PersonalSettingsComponent, ProfileSettingsComponent, SecuritySettingsComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		RouterModule.forChild(routes),
		PageComponent,
		InputComponent,
		ButtonComponent,
		CardComponent,
		EmptyStateComponent,
		DeleteModalComponent,
		ModalComponent,
		CopyButtonComponent,
		SkeletonLoaderComponent,
		TagComponent,
		StatusColorModule,
		SelectComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent
	]
})
export class AccountModule {}
