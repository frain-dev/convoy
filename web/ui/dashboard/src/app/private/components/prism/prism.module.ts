import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismComponent } from './prism.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@NgModule({
	declarations: [PrismComponent],
	imports: [CommonModule, ButtonComponent],
	exports: [PrismComponent]
})
export class PrismModule {}
