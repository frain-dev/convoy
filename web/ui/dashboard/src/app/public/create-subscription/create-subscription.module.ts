import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSubscriptionComponent } from './create-subscription.component';
import { RouterModule, Routes } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { DialogDirective } from 'src/app/components/modal/modal.component';

const routes: Routes = [{ path: '', component: CreateSubscriptionComponent }];

@NgModule({
	declarations: [CreateSubscriptionComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateSubscriptionModule, DialogDirective]
})
export class CreateSubscriptionPublicModule {}
