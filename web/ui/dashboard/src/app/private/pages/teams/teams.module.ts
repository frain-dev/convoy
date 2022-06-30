import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TeamsComponent } from './teams.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { TableLoaderModule } from '../../components/table-loader/table-loader.module';
import { DeleteModalModule } from '../../components/delete-modal/delete-modal.module';

const routes: Routes = [
	{ path: '', component: TeamsComponent },
	{ path: 'new', component: TeamsComponent }
];

@NgModule({
	declarations: [TeamsComponent],
	imports: [CommonModule, FormsModule, TableLoaderModule, ReactiveFormsModule, DeleteModalModule, RouterModule.forChild(routes)]
})
export class TeamsModule {}
