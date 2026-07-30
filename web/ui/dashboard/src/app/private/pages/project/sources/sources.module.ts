import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { SourcesComponent } from './sources.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from 'src/app/private/components/create-source/create-source.module';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { SourceValueModule } from 'src/app/pipes/source-value/source-value.module';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';

const routes: Routes = [{ path: '', component: SourcesComponent }];

@NgModule({
	declarations: [SourcesComponent],
	imports: [CommonModule, FormsModule, RouterModule.forChild(routes), CreateSourceModule, DeleteModalComponent, SourceValueModule, PermissionDirective, DialogDirective]
})
export class SourcesModule {}
