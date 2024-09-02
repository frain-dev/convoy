import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project-component.component';
import { ReactiveFormsModule } from '@angular/forms';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DialogHeaderComponent, DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TokenModalComponent } from '../token-modal/token-modal.component';
import { PermissionDirective } from '../permission/permission.directive';
import { NotificationComponent } from 'src/app/components/notification/notification.component';
import { ConfigButtonComponent } from '../config-button/config-button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { UploadEventsComponent } from '../upload-events/upload-events.component';
import { EventCatalogueComponent } from '../event-catalogue/event-catalogue.component';

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		TooltipComponent,
		RadioComponent,
		ToggleComponent,

		SelectComponent,
		ButtonComponent,
		DialogHeaderComponent,
		CopyButtonComponent,
		CardComponent,
		ButtonComponent,
		TooltipComponent,
		TableCellComponent,
		TableComponent,
		TableHeadCellComponent,
		TableHeadComponent,
		TableRowComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent,
		TokenModalComponent,
		PermissionDirective,
		DialogDirective,
		NotificationComponent,
		ConfigButtonComponent,
        TagComponent,
        UploadEventsComponent,
        EventCatalogueComponent
	],
	exports: [CreateProjectComponent]
})
export class CreateProjectComponentModule {}
