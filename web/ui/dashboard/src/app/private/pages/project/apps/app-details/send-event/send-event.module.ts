import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SendEventComponent } from './send-event.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ReactiveFormsModule } from '@angular/forms';



@NgModule({
  declarations: [SendEventComponent],
  imports: [
    CommonModule, ModalComponent, SelectComponent, InputComponent, ButtonComponent, ReactiveFormsModule
  ],
  exports: [SendEventComponent]
})
export class SendEventModule { }
