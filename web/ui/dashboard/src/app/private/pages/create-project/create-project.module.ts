import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from '../../components/create-source/create-source.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CreateAppModule } from '../../components/create-app/create-app.module';
import { CreateProjectComponentModule } from '../../components/create-project-component/create-project-component.module';
import { CreateSubscriptionModule } from '../../components/create-subscription/create-subscription.module';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

const routes: Routes = [{ path: '', component: CreateProjectComponent }];

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, CreateAppModule, RouterModule.forChild(routes), CreateSourceModule, CreateProjectComponentModule, CreateSubscriptionModule, ModalComponent, CardComponent, TooltipComponent, ButtonComponent]
})
export class CreateProjectModule {}
