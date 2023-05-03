import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SettingsComponent } from './settings.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateProjectComponentModule } from 'src/app/private/components/create-project-component/create-project-component.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';

const routes: Routes = [{ path: '', component: SettingsComponent }];

@NgModule({
	declarations: [SettingsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateProjectComponentModule, LoaderModule, DeleteModalComponent, CardComponent, ButtonComponent, PermissionDirective]
})
export class SettingsModule {}
