import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from '../../components/create-source/create-source.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from '../../components/tooltip/tooltip.module';
import { CreateAppModule } from '../../components/create-app/create-app.module';

const routes: Routes = [{ path: '', component: CreateProjectComponent }];

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, TooltipModule, CreateAppModule, RouterModule.forChild(routes), CreateSourceModule]
})
export class CreateProjectModule {}
