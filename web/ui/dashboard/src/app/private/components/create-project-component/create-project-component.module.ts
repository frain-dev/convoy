import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project-component.component';
import { ReactiveFormsModule } from '@angular/forms';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TokenModalComponent } from '../token-modal/token-modal.component';

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
		ModalComponent,
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
        TokenModalComponent
	],
	exports: [CreateProjectComponent]
})
export class CreateProjectComponentModule {}
