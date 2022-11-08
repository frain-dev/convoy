import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project-component.component';
import { ReactiveFormsModule } from '@angular/forms';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { InputComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ConfirmationModalComponent } from '../confirmation-modal/confirmation-modal.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { CardComponent } from 'src/app/components/card/card.component';

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		TooltipComponent,
		RadioComponent,
		ToggleComponent,
		InputComponent,
		SelectComponent,
		ButtonComponent,
		ModalComponent,
		CopyButtonComponent,
		ConfirmationModalComponent,
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
		LabelComponent
	],
	exports: [CreateProjectComponent]
})
export class CreateProjectComponentModule {}
