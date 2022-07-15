import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OrganisationComponent } from './organisation.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { DeleteModalModule } from '../../components/delete-modal/delete-modal.module';
import { PageComponent } from 'src/app/components/page/page.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

const routes: Routes = [{ path: '', component: OrganisationComponent }];

@NgModule({
	declarations: [OrganisationComponent],
	imports: [CommonModule, ReactiveFormsModule, DeleteModalModule, PageComponent, CardComponent, InputComponent, ButtonComponent, RouterModule.forChild(routes)]
})
export class OrganisationModule {}
