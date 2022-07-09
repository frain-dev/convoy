import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SubscriptionsComponent } from './subscriptions.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';

const routes: Routes = [{ path: '', component: SubscriptionsComponent }];

@NgModule({
	declarations: [SubscriptionsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateSubscriptionModule, DeleteModalModule, ButtonComponent, ModalComponent]
})
export class SubscriptionsModule {}
