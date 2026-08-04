import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SubscriptionsComponent } from './subscriptions.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DialogHeaderComponent, DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { CardComponent } from 'src/app/components/card/card.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { SourceValueModule } from 'src/app/pipes/source-value/source-value.module';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { FormsModule } from '@angular/forms';

const routes: Routes = [{ path: '', component: SubscriptionsComponent }];

@NgModule({
	declarations: [SubscriptionsComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		CreateSubscriptionModule,
		ButtonComponent,
		DialogHeaderComponent,
		CardComponent,
		TagComponent,
		CopyButtonComponent,
		DeleteModalComponent,
		SourceValueModule,
		PermissionDirective,
		DropdownComponent,
		DropdownOptionDirective,
		DialogDirective,
		FormsModule
	]
})
export class SubscriptionsModule {}
