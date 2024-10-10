import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SubscriptionsComponent } from './subscriptions.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DialogHeaderComponent, DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { CardComponent } from 'src/app/components/card/card.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { FormatSecondsPipe } from 'src/app/pipes/formatSeconds/format-seconds.pipe';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { SourceValueModule } from 'src/app/pipes/source-value/source-value.module';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { TableCellComponent, TableComponent, TableRowComponent } from 'src/app/components/table/table.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { FormsModule } from '@angular/forms';
import { EndpointFilterComponent } from 'src/app/private/components/endpoints-filter/endpoints-filter.component';

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
		ListItemComponent,
		TagComponent,
		EmptyStateComponent,
		CopyButtonComponent,
		FormatSecondsPipe,
		DeleteModalComponent,
		SourceValueModule,
		TooltipComponent,
		CardComponent,
		PaginationComponent,
		PermissionDirective,
		TableCellComponent,
		TableRowComponent,
		TableComponent,
		DropdownComponent,
		DropdownOptionDirective,
		LoaderModule,
        DialogDirective,
        FormsModule,
        EndpointFilterComponent
	]
})
export class SubscriptionsModule {}
