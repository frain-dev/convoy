import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TeamsComponent } from './teams.component';
import { FormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { TableLoaderModule } from '../../components/table-loader/table-loader.module';
import { CreateProjectComponentModule } from '../../components/create-project-component/create-project-component.module';
import { AddTeamMemberModule } from '../../components/add-team-member/add-team-member.module';

const routes: Routes = [
	{ path: '', component: TeamsComponent },
	{ path: 'new', component: TeamsComponent },
	{ path: 'new/project', component: TeamsComponent }
];

@NgModule({
	declarations: [TeamsComponent],
	imports: [CommonModule, FormsModule, TableLoaderModule, CreateProjectComponentModule, AddTeamMemberModule, RouterModule.forChild(routes)]
})
export class TeamsModule {}
