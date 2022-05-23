import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from '../../components/create-source/create-source.module';

const routes: Routes = [{ path: '', component: CreateProjectComponent }];

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateSourceModule]
})
export class CreateProjectModule {}
