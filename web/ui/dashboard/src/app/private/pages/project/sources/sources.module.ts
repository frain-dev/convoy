import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SourcesComponent } from './sources.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from 'src/app/private/components/create-source/create-source.module';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { SourceValueModule } from 'src/app/pipes/source-value/source-value.module';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';

const routes: Routes = [{ path: '', component: SourcesComponent }];

@NgModule({
	declarations: [SourcesComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		CreateSourceModule,
		DeleteModalComponent,
		TagComponent,
		ButtonComponent,
		ListItemComponent,
		EmptyStateComponent,
		CardComponent,
		ModalComponent,
		ModalHeaderComponent,
		CopyButtonComponent,
		SourceValueModule,
		DropdownComponent,
		DropdownOptionDirective,
		SkeletonLoaderComponent,
		TooltipComponent,
		PaginationComponent
	]
})
export class SourcesModule {}
