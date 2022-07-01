import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OrganisationComponent } from './organisation.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { DeleteModalModule } from '../../components/delete-modal/delete-modal.module';

const routes: Routes = [{ path: '', component: OrganisationComponent }];

@NgModule({
	declarations: [OrganisationComponent],
	imports: [CommonModule, ReactiveFormsModule, DeleteModalModule, RouterModule.forChild(routes)]
})
export class OrganisationModule {}
