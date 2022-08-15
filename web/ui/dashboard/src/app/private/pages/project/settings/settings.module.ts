import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SettingsComponent } from './settings.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateProjectComponentModule } from 'src/app/private/components/create-project-component/create-project-component.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';
import { CardComponent } from 'src/stories/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

const routes: Routes = [{ path: '', component: SettingsComponent }];

@NgModule({
	declarations: [SettingsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateProjectComponentModule, LoaderModule, DeleteModalModule, CardComponent, ButtonComponent]
})
export class SettingsModule {}
