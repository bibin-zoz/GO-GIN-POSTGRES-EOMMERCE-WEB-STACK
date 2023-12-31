package handlers

import (
	db "ecommercestore/database"
	"ecommercestore/helpers"
	"ecommercestore/models"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/nfnt/resize"
)

func ProductsHandler(c *gin.Context) {

	var products []models.Products
	var category []models.Categories
	var brand []models.Brands

	pgno, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		pgno = 1
	}
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 10
	}
	offset := (pgno - 1) * limit

	db.DB.Find(&category)
	db.DB.Find(&brand)
	var count int64
	db.DB.Model(models.Products{}).Count(&count)
	fmt.Println("count", count)
	if err := db.DB.Preload("Variants").Preload("Category").Preload("Images").Preload("Brand").Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		fmt.Println("failed to load products")
	}
	if count%2 != 0 {
		count = count + 1
	}
	fmt.Println("count", count)
	fmt.Println("lim", limit)
	num := int(count) / (limit)
	if int(count)%limit != 0 {
		num = num + 1
	}
	fmt.Println("num", num)
	pagenumber := make([]int, 0)

	for i := 1; i <= num; i++ {
		pagenumber = append(pagenumber, i)
	}
	if len(pagenumber) == 0 {
		pagenumber = append(pagenumber, 1)
	}
	fmt.Println("pagenumber", len(pagenumber))

	// c.JSON(http.StatusOK, products)
	fmt.Println("pgno", pgno)

	c.HTML(http.StatusOK, "productmanage.html", gin.H{
		"Products":    products,
		"Category":    category,
		"Brands":      brand,
		"Pagenumber":  pagenumber,
		"Entries":     limit,
		"Currentpage": pgno,
	})

}

// add product
func AddProduct(c *gin.Context) {

	var Producterror models.Producterror

	productName := c.PostForm("productName")
	productDetails := c.PostForm("productDetails")
	storage := c.PostForm("storage")
	ram := c.PostForm("ram")
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	categoryID, _ := strconv.Atoi(c.PostForm("categoryID"))
	brandID, _ := strconv.Atoi(c.PostForm("brandID"))

	if productName == "" || productDetails == "" || stock <= 0 || price <= 100 {
		if productName == "" {
			Producterror.ProductNameError = "invalid productname"
		}
		if productDetails == "" {
			Producterror.ProductDetailsError = "Details Cannot be empty"
		}
		if stock <= 0 {
			Producterror.StockError = "Enter Valid Stock Number"
		}
		if math.IsNaN(price) || price <= 100 {
			Producterror.PriceError = "Invalid price"
			fmt.Println(Producterror)

		}
		c.JSON(http.StatusBadRequest, gin.H{
			"Errors": Producterror,
		})
		fmt.Println(Producterror)
		return

	}
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	files := c.Request.MultipartForm.File["images"]

	newProduct := models.Products{
		ProductName:    productName,
		ProductDetails: productDetails,
		// Stock:          stock,
		// Price:          price,
		CategoryID: uint(categoryID),
		BrandID:    uint(brandID),
	}

	result := db.DB.Create(&newProduct)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	newProductVariant := models.ProductVariants{
		ProductID: newProduct.ID,
		Processor: c.PostForm("processor"),
		Storage:   storage,
		Ram:       ram,
		Status:    "listed",
		Stock:     stock,
		MaxPrice:  price,
	}
	newProductVariant.CreateSlug(productName)

	resultVariant := db.DB.Create(&newProductVariant)
	if resultVariant.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": resultVariant.Error.Error()})
		return
	}

	for _, file := range files {
		isValid, detectedType := helpers.IsImageFile(file)
		if !isValid {
			fmt.Println("Unknown format:", detectedType)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid file format. Only image files are allowed."})
			return
		}

		fmt.Println("Valid format:", detectedType)

		src, err := file.Open()
		if err != nil {
			fmt.Println("Error opening file:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer src.Close()

		newImage := models.Image{
			ProductID: newProduct.ID,
		}

		imageResult := db.DB.Create(&newImage)
		if imageResult.Error != nil {
			fmt.Println("Error creating new image record:", imageResult.Error)
			c.JSON(http.StatusInternalServerError, gin.H{"error": imageResult.Error.Error()})
			return
		}

		filename := fmt.Sprintf("%d_%s", newImage.ID, file.Filename)
		filepath := filepath.Join("static/uploads", filename)
		filepath = strings.ReplaceAll(filepath, "\\", "/")
		dst, err := os.Create(filepath)
		if err != nil {
			fmt.Println("Error creating destination file:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer dst.Close()

		img, _, err := image.Decode(src)
		if err != nil {
			fmt.Println("Error decoding image:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		resizedImg := resize.Resize(465, 576, img, resize.Lanczos3)

		if err := jpeg.Encode(dst, resizedImg, nil); err != nil {
			fmt.Println("Error encoding image:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		newImage.FilePath = filepath
		db.DB.Save(&newImage)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Product added successfully",
		"product":  newProduct,
		"redirect": "/admin/products",
	})

}

func UpdateProductStatus(c *gin.Context) {

	var req DeleteRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ID := req.ID

	var product models.Products
	if err := db.DB.Where("id = ?", ID).First(&product).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	if product.Status == "listed" {
		product.Status = "unlisted"
	} else {
		product.Status = "listed"
	}

	if err := db.DB.Save(&product).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Product Status Updated Successfully",
		"redirect": "/admin/categories",
	})
}

func DeleteProductHandler(c *gin.Context) {

	var req DeleteRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	productID := req.ID
	fmt.Println(productID)

	var product models.Products

	if err := db.DB.Preload("Images").First(&product, productID).Error; err != nil {
		return
	}

	// Delete the product and its associated images
	if err := db.DB.Delete(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "product deleted successfully"})
}

func ProductDetailsHandler(c *gin.Context) {
	ID, _ := strconv.Atoi(c.Query("id"))
	var Product models.Products
	var category []models.Categories
	if err := db.DB.Preload("Brand").Preload("Variants").Preload("Images").Find(&Product, ID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Product not found"})
		return
	}

	if err := db.DB.Find(&category).Error; err != nil {

		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Product not found"})
		return
	}

	log.Println(category)

	c.HTML(http.StatusOK, "productedit.html", gin.H{
		"Product":  Product,
		"Category": category,
	})

	// // Return the result as JSON
	// if c.Request.Header.Get("Accept") == "application/json" {
	// 	// Return JSON if the client accepts JSON
	// 	c.JSON(http.StatusOK, gin.H{"Products": product})
	// } else {
	// 	// Return HTML if the client accepts HTML or doesn't specify a preference
	// 	c.HTML(http.StatusOK, "productedit.html", gin.H{
	// 		"Products": product,
	// 	})
	// }

}

func ProductUpdateHandler(c *gin.Context) {
	// Get product ID from the form data
	idStr := c.PostForm("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Parse other form data
	variantID, err := strconv.Atoi(c.PostForm("variantID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variant ID"})
		return
	}

	productName := c.PostForm("productName")
	if productName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product name cannot be empty"})
		return
	}

	productDetails := c.PostForm("productDetails")
	if productDetails == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product details cannot be empty"})
		return
	}
	discount, err := strconv.Atoi(c.PostForm("productDiscount"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Discount price"})
		return
	}

	categoryID, err := strconv.ParseUint(c.PostForm("categoryID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}
	storage := c.PostForm("storage")
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	price, err := strconv.ParseFloat(c.PostForm("price"), 64)
	addProduct := c.PostForm("addProduct")
	ram := c.PostForm("ram")
	fmt.Println("variant", variantID)

	if variantID == 0 && addProduct == "true" {
		fmt.Println("hii")
		if ram == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ram  cannot be empty"})
			return
		}

		if storage == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storage  cannot be empty"})
			return
		}

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stock value"})
			return
		}

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price value"})
			return
		}

		fmt.Println("variant zero entered")
		// VariantID is 0, create a new ProductVariants record
		newProductVariant := models.ProductVariants{
			ProductID: uint(id),
			Processor: "tempprocessor", // Assuming you get the processor value from the form
			Storage:   storage,
			Ram:       ram,
			Status:    "listed",
			Stock:     stock, // You can set the default value here or get it from the form
			MaxPrice:  price, // Assuming max price is the same as the regular price for now
		}
		newProductVariant.CreateSlug(productName)

		// Insert the new ProductVariant into the database
		result := db.DB.Create(&newProductVariant)
		if result.Error != nil {
			fmt.Println("result.Error.Error()")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Variant already Exist"})
			return
		}
		if result.Error != nil {
			fmt.Println("Error creating ProductVariants:", result.Error.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		fmt.Println("variant zero existed")
		c.JSON(http.StatusOK, gin.H{
			"message":  "variant added successfully",
			"redirect": "/admin/product?id=" + idStr,
		})
		return
	}

	result := db.DB.Model(&models.Products{}).Where("id=?", id).Updates(models.Products{
		ProductName:    productName,
		ProductDetails: productDetails,
		CategoryID:     uint(categoryID),
		Discount:       uint(discount),
	})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		log.Println("prodcut model error")

		return
	}
	helpers.UpdateDiscountPrice()

	slugInput := fmt.Sprintf("%s-%s-%s", productName, storage, ram)
	newSlug := slug.MakeLang(slugInput, "en")
	db.DB.Where("slug=?", newSlug)

	result = db.DB.Model(&models.ProductVariants{}).Where("id=?", variantID).Updates(models.ProductVariants{
		Storage: storage,
		Ram:     ram,
		Stock:   stock,
		Price:   price,
		Slug:    newSlug,
	})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	deleteImageIDs := c.PostFormArray("deleteImages")
	fmt.Println("Delete Image IDs:", deleteImageIDs)

	for _, deleteImageIDStr := range deleteImageIDs {
		fmt.Println("Deleting Image ID:", deleteImageIDStr)
		deleteImageID, err := strconv.Atoi(deleteImageIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
			return
		}

		deleteResult := db.DB.Delete(&models.Image{}, deleteImageID)
		if deleteResult.Error != nil {
			fmt.Println("Error deleting image:", deleteResult.Error.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": deleteResult.Error.Error()})
			return
		}

	}

	err = c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	files := c.Request.MultipartForm.File["images"]

	for _, file := range files {
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer src.Close()

		newImage := models.Image{
			ProductID: uint(id),
		}

		imageResult := db.DB.Create(&newImage)

		if imageResult.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": imageResult.Error.Error()})
			return
		}

		// Resize the image
		resizedImage, err := helpers.ResizeImage(src, 500, 500)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error resizing image: " + err.Error()})
			return
		}

		filename := fmt.Sprintf("%d_%s", newImage.ID, file.Filename)
		filepath := filepath.Join("static/uploads", filename)
		filepath = strings.ReplaceAll(filepath, "\\", "/")
		dst, err := os.Create(filepath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer dst.Close()

		if err := helpers.SaveResizedImage(dst, resizedImage, "jpeg"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		newImage.FilePath = filepath
		db.DB.Save(&newImage)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Product updated successfully",
		"redirect": "/admin/product?id=" + idStr,
	})
}
