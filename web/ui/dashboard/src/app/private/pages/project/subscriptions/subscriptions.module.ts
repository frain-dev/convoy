import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SubscriptionsComponent } from './subscriptions.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { FormatSecondsPipe } from 'src/app/pipes/formatSeconds/format-seconds.pipe';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';

const routes: Routes = [{ path: '', component: SubscriptionsComponent }];

@NgModule({
	declarations: [SubscriptionsComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateSubscriptionModule, StatusColorModule, ButtonComponent, ModalComponent, CardComponent, ListItemComponent, TagComponent, EmptyStateComponent, CopyButtonComponent, FormatSecondsPipe, DeleteModalComponent]
})
export class SubscriptionsModule {}
