import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectsComponent } from './projects.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateOrganisationModule } from '../../components/create-organisation/create-organisation.module';
import { LoaderModule } from '../../components/loader/loader.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { PageDirective } from 'src/app/components/page/page.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { PermissionDirective } from '../../components/permission/permission.directive';

const routes: Routes = [{ path: '', component: ProjectsComponent }];

@NgModule({
	declarations: [ProjectsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateOrganisationModule, LoaderModule, ButtonComponent, ModalComponent, PageDirective, EmptyStateComponent, CardComponent, PermissionDirective]
})
export class ProjectsModule {}
