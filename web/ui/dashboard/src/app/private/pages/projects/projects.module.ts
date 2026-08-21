import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { ProjectsComponent } from './projects.component';
import { Routes, RouterModule } from '@angular/router';
import { LoaderModule } from '../../components/loader/loader.module';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { InputErrorComponent } from 'src/app/components/input/input.component';
import { TokenModalComponent } from '../../components/token-modal/token-modal.component';
import { PermissionDirective } from '../../components/permission/permission.directive';
import { TrialModalComponent } from '../settings/billing/trial-modal.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';

const routes: Routes = [{ path: '', component: ProjectsComponent }];

@NgModule({
	declarations: [ProjectsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ReactiveFormsModule, LoaderModule, DialogDirective, InputErrorComponent, TokenModalComponent, PermissionDirective, TrialModalComponent, TooltipComponent]
})
export class ProjectsModule {}
