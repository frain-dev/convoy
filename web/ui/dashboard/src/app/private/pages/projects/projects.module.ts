import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectsComponent } from './projects.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateOrganisationModule } from '../../components/create-organisation/create-organisation.module';
import { LoaderModule } from '../../components/loader/loader.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';

const routes: Routes = [{ path: '', component: ProjectsComponent }];

@NgModule({
	declarations: [ProjectsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateOrganisationModule, LoaderModule, ButtonComponent, ModalComponent]
})
export class ProjectsModule {}
