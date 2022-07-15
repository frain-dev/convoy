import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DeleteModalComponent } from './delete-modal.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@NgModule({
	declarations: [DeleteModalComponent],
	imports: [CommonModule, ModalComponent, ButtonComponent],
	exports: [DeleteModalComponent]
})
export class DeleteModalModule {}
