import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateProjectComponentModule } from '../../components/create-project-component/create-project-component.module';
import { DialogHeaderComponent, DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from '../../components/loader/loader.module';

const routes: Routes = [{ path: '', component: CreateProjectComponent }];

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateProjectComponentModule, DialogDirective, ButtonComponent, LoaderModule, DialogHeaderComponent]
})
export class CreateProjectModule {}
